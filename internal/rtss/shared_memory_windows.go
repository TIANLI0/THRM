//go:build windows

package rtss

import (
	"encoding/binary"
	"fmt"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	sharedMemoryName = "RTSSSharedMemoryV2"
	ownerName        = "THRM"
	rtssSignature    = 0x52545353 // C multi-character constant 'RTSS'
	mapReadWrite     = 0x0002 | 0x0004
	maxViewSize      = 64 << 20

	headerSize         = 40
	busyOffset         = 36
	osdFrameOffset     = 32
	osdTextOffset      = 0
	osdOwnerOffset     = 256
	osdExtendedOffset  = 512
	osdTextSize        = 256
	osdOwnerSize       = 256
	osdExtendedSize    = 4096
	osdBufferSize      = 262144
	osdExtended2Offset = osdExtendedOffset + osdExtendedSize + osdBufferSize
	osdExtended2Size   = 32768

	rtssVersionExtended  = 0x00020007
	rtssVersionExtended2 = 0x00020014

	releaseLockAttempts   = 10
	releaseLockRetryDelay = time.Millisecond
)

var procOpenFileMappingW = windows.NewLazySystemDLL("kernel32.dll").NewProc("OpenFileMappingW")

type sharedMemoryLayout struct {
	version      uint32
	entrySize    int
	arrayOffset  int
	arraySize    int
	requiredSize int
}

type sharedMemorySink struct {
	mapHandle    windows.Handle
	viewBase     uintptr
	view         []byte
	entry        int
	positionMode string
	positionX    int
	positionY    int
}

func newSharedMemorySink() osdSink {
	return &sharedMemorySink{entry: -1, positionMode: "anchor"}
}

func (s *sharedMemorySink) SetPosition(mode string, x, y int) {
	if mode != "custom" {
		mode = "anchor"
	}
	s.positionMode = mode
	s.positionX = x
	s.positionY = y
}

func (s *sharedMemorySink) Update(rpm uint16) bool {
	if !s.ensureMapped() {
		return false
	}
	layout, ok := decodeLayout(s.view, len(s.view))
	if !ok {
		s.unmap()
		return false
	}

	busy, ok := s.tryLock(layout)
	if !ok {
		return false
	}
	if busy != nil {
		defer unlock(busy)
	}

	if s.entry < 1 || s.entry >= layout.arraySize || !s.ownedEntry(layout, s.entry) {
		s.entry = s.findEntry(layout)
	}
	if s.entry < 1 {
		return false
	}

	entry := s.entryBytes(layout, s.entry)
	writeOSDText(entry, layout.version, formatOSDTextAt(rpm, s.positionMode, s.positionX, s.positionY))
	atomic.AddUint32((*uint32)(unsafe.Pointer(&s.view[osdFrameOffset])), 1)
	return true
}

func writeOSDText(entry []byte, version uint32, text string) {
	switch {
	case version >= rtssVersionExtended2 && len(entry) >= osdExtended2Offset+osdExtended2Size:
		writeCString(entry[osdExtended2Offset:], osdExtended2Size, text)
	case version >= rtssVersionExtended && len(entry) >= osdExtendedOffset+osdExtendedSize:
		writeCString(entry[osdExtendedOffset:], osdExtendedSize, text)
	default:
		writeCString(entry[osdTextOffset:], osdTextSize, text)
	}
}

func formatOSDText(rpm uint16) string {
	return formatOSDTextAt(rpm, "anchor", 0, 0)
}

func formatOSDTextAt(rpm uint16, positionMode string, x, y int) string {
	position := ""
	if positionMode == "custom" {
		position = fmt.Sprintf("<P=%d,%d>", x, y)
	}
	// RTSS concatenates client slots into one hypertext stream. Reset styles so
	// THRM does not inherit the preceding client's color or font size.
	return fmt.Sprintf("%s<C><S>Cooler Fan: %d RPM", position, rpm)
}

func (s *sharedMemorySink) findEntry(layout sharedMemoryLayout) int {
	target := -1
	for i := layout.arraySize - 1; i >= 1; i-- {
		entry := s.entryBytes(layout, i)
		owner := readCString(entry[osdOwnerOffset : osdOwnerOffset+osdOwnerSize])
		if owner == "" || owner == ownerName {
			target = i
			break
		}
	}
	if target < 1 {
		return -1
	}

	entry := s.entryBytes(layout, target)
	clear(entry)
	writeCString(entry[osdOwnerOffset:], osdOwnerSize, ownerName)
	if !s.ownedEntry(layout, target) {
		return -1
	}

	// A previous THRM process may have left an owned slot behind. Keep only the
	// highest usable slot so OverlayEditor's final positioning tags render first.
	for i := 1; i < layout.arraySize; i++ {
		if i != target && s.ownedEntry(layout, i) {
			clear(s.entryBytes(layout, i))
		}
	}
	return target
}

func (s *sharedMemorySink) ownedEntry(layout sharedMemoryLayout, index int) bool {
	entry := s.entryBytes(layout, index)
	return readCString(entry[osdOwnerOffset:osdOwnerOffset+osdOwnerSize]) == ownerName
}

func (s *sharedMemorySink) entryBytes(layout sharedMemoryLayout, index int) []byte {
	start := layout.arrayOffset + index*layout.entrySize
	return s.view[start : start+layout.entrySize]
}

func (s *sharedMemorySink) tryLock(layout sharedMemoryLayout) (*uint32, bool) {
	return s.lockWithRetry(layout, 1, nil)
}

func (s *sharedMemorySink) lockWithRetry(layout sharedMemoryLayout, attempts int, wait func()) (*uint32, bool) {
	if layout.version < 0x0002000e {
		return nil, true
	}
	busy := (*uint32)(unsafe.Pointer(&s.view[busyOffset]))
	return busy, tryLockWithRetry(busy, attempts, wait)
}

func decodeLayout(view []byte, maxSize int) (sharedMemoryLayout, bool) {
	if len(view) < headerSize || maxSize < headerSize || binary.LittleEndian.Uint32(view[:4]) != rtssSignature {
		return sharedMemoryLayout{}, false
	}
	version := binary.LittleEndian.Uint32(view[4:8])
	entrySize := uint64(binary.LittleEndian.Uint32(view[20:24]))
	arrayOffset := uint64(binary.LittleEndian.Uint32(view[24:28]))
	arraySize := uint64(binary.LittleEndian.Uint32(view[28:32]))
	viewSize := uint64(maxSize)
	if version < 0x00020000 || entrySize < osdOwnerOffset+osdOwnerSize || arrayOffset < headerSize || arraySize <= 1 {
		return sharedMemoryLayout{}, false
	}
	if arrayOffset > viewSize || entrySize > (viewSize-arrayOffset)/arraySize {
		return sharedMemoryLayout{}, false
	}
	requiredSize := arrayOffset + entrySize*arraySize
	if requiredSize > viewSize {
		return sharedMemoryLayout{}, false
	}
	return sharedMemoryLayout{
		version:      version,
		entrySize:    int(entrySize),
		arrayOffset:  int(arrayOffset),
		arraySize:    int(arraySize),
		requiredSize: int(requiredSize),
	}, true
}

func tryLock(value *uint32) bool {
	for {
		old := atomic.LoadUint32(value)
		if old&1 != 0 {
			return false
		}
		if atomic.CompareAndSwapUint32(value, old, old|1) {
			return true
		}
	}
}

func tryLockWithRetry(value *uint32, attempts int, wait func()) bool {
	for attempt := 0; attempt < attempts; attempt++ {
		if tryLock(value) {
			return true
		}
		if attempt+1 < attempts && wait != nil {
			wait()
		}
	}
	return false
}

func unlock(value *uint32) { atomic.AndUint32(value, ^uint32(1)) }

func readCString(value []byte) string {
	for i, b := range value {
		if b == 0 {
			return string(value[:i])
		}
	}
	return string(value)
}

func writeCString(dst []byte, size int, value string) {
	if len(dst) < size {
		return
	}
	clear(dst[:size])
	copy(dst[:size-1], value)
}

type mappedRegionInfo struct {
	baseAddress    uintptr
	allocationBase uintptr
	regionSize     uintptr
	state          uint32
	protect        uint32
}

func queryMappedRegion(view uintptr) (mappedRegionInfo, bool) {
	if view == 0 {
		return mappedRegionInfo{}, false
	}
	var info windows.MemoryBasicInformation
	if unsafe.Sizeof(uintptr(0)) == 4 {
		// VirtualQuery uses the traditional 28-byte MEMORY_BASIC_INFORMATION
		// layout in 32-bit processes, while x/sys exposes the newer structure
		// containing PartitionId. Read the returned buffer at its native offsets.
		const info32Size = 28
		if err := windows.VirtualQuery(view, &info, info32Size); err != nil {
			return mappedRegionInfo{}, false
		}
		raw := unsafe.Slice((*byte)(unsafe.Pointer(&info)), info32Size)
		return mappedRegionInfo{
			baseAddress:    uintptr(binary.LittleEndian.Uint32(raw[0:4])),
			allocationBase: uintptr(binary.LittleEndian.Uint32(raw[4:8])),
			regionSize:     uintptr(binary.LittleEndian.Uint32(raw[12:16])),
			state:          binary.LittleEndian.Uint32(raw[16:20]),
			protect:        binary.LittleEndian.Uint32(raw[20:24]),
		}, true
	}
	if err := windows.VirtualQuery(view, &info, unsafe.Sizeof(info)); err != nil {
		return mappedRegionInfo{}, false
	}
	return mappedRegionInfo{
		baseAddress:    info.BaseAddress,
		allocationBase: info.AllocationBase,
		regionSize:     info.RegionSize,
		state:          info.State,
		protect:        info.Protect,
	}, true
}

// mappedViewSize returns a conservative bound for slices created from the
// mapped address. The shared-memory header is untrusted until this succeeds.
func mappedViewSize(view uintptr) (int, bool) {
	info, ok := queryMappedRegion(view)
	if !ok {
		return 0, false
	}
	if info.baseAddress != view || info.allocationBase != view || info.state != windows.MEM_COMMIT {
		return 0, false
	}
	if info.protect&(windows.PAGE_NOACCESS|windows.PAGE_GUARD) != 0 || info.regionSize < uintptr(headerSize) {
		return 0, false
	}
	size := info.regionSize
	if size > uintptr(maxViewSize) {
		size = uintptr(maxViewSize)
	}
	return int(size), true
}

// mappedViewBytes is the only place that converts the address returned by
// MapViewOfFile into a Go slice. The conversion is safe only while all of the
// following conditions remain true:
//   - view is the unchanged base address returned by MapViewOfFile;
//   - VirtualQuery has verified a committed, accessible region of regionSize;
//   - length does not exceed that verified region; and
//   - the mapping remains alive until every returned slice is no longer used.
//
// The caller owns the mapping lifetime. This helper enforces the local address
// and length checks so future call sites cannot accidentally create an
// out-of-bounds slice from an untrusted RTSS header.
func mappedViewBytes(view uintptr, length, regionSize int) ([]byte, bool) {
	if view == 0 || length <= 0 || regionSize <= 0 || length > regionSize {
		return nil, false
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(view)), length), true
}

func (s *sharedMemorySink) ensureMapped() bool {
	if len(s.view) > 0 {
		return true
	}
	name := windows.StringToUTF16Ptr(sharedMemoryName)
	raw, _, _ := procOpenFileMappingW.Call(uintptr(mapReadWrite), 0, uintptr(unsafe.Pointer(name)))
	if raw == 0 {
		return false
	}
	h := windows.Handle(raw)
	view, err := windows.MapViewOfFile(h, mapReadWrite, 0, 0, 0)
	if err != nil {
		windows.CloseHandle(h)
		return false
	}

	viewSize, ok := mappedViewSize(view)
	if !ok {
		windows.UnmapViewOfFile(view)
		windows.CloseHandle(h)
		return false
	}
	header, ok := mappedViewBytes(view, headerSize, viewSize)
	if !ok {
		windows.UnmapViewOfFile(view)
		windows.CloseHandle(h)
		return false
	}
	layout, ok := decodeLayout(header, viewSize)
	if !ok {
		windows.UnmapViewOfFile(view)
		windows.CloseHandle(h)
		return false
	}
	mapped, ok := mappedViewBytes(view, layout.requiredSize, viewSize)
	if !ok {
		windows.UnmapViewOfFile(view)
		windows.CloseHandle(h)
		return false
	}
	s.mapHandle = h
	s.viewBase = view
	s.view = mapped
	return true
}

func (s *sharedMemorySink) unmap() {
	if s.viewBase != 0 {
		windows.UnmapViewOfFile(s.viewBase)
		s.viewBase = 0
	}
	s.view = nil
	if s.mapHandle != 0 {
		windows.CloseHandle(s.mapHandle)
		s.mapHandle = 0
	}
	s.entry = -1
}

func (s *sharedMemorySink) Close() {
	if len(s.view) == 0 {
		return
	}
	layout, ok := decodeLayout(s.view, len(s.view))
	if ok {
		s.releaseEntry(layout)
	}
	s.unmap()
}

func waitForReleaseLock() { time.Sleep(releaseLockRetryDelay) }

func (s *sharedMemorySink) releaseEntry(layout sharedMemoryLayout) {
	s.releaseEntryWithWait(layout, waitForReleaseLock)
}

// RTSS holds the busy lock briefly while consuming OSD updates. Normal
// publishing stays non-blocking, while shutdown retries for a bounded period
// so a transient collision does not leave THRM's last text in the slot.
func (s *sharedMemorySink) releaseEntryWithWait(layout sharedMemoryLayout, wait func()) bool {
	busy, ok := s.lockWithRetry(layout, releaseLockAttempts, wait)
	if !ok {
		return false
	}
	if busy != nil {
		defer unlock(busy)
	}
	if s.entry > 0 && s.entry < layout.arraySize && s.ownedEntry(layout, s.entry) {
		clear(s.entryBytes(layout, s.entry))
		atomic.AddUint32((*uint32)(unsafe.Pointer(&s.view[osdFrameOffset])), 1)
	}
	return true
}
