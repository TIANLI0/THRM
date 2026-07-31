//go:build windows

package rtss

import (
	"encoding/binary"
	"fmt"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	sharedMemoryName = "RTSSSharedMemoryV2"
	ownerName        = "THRM"
	rtssSignature    = 0x52545353 // C multi-character constant 'RTSS'
	mapReadWrite     = 0x0002 | 0x0004
	maxViewSize      = 64 << 20

	headerSize        = 40
	busyOffset        = 36
	osdFrameOffset    = 32
	osdTextOffset     = 0
	osdOwnerOffset    = 256
	osdExtendedOffset = 512
	osdTextSize       = 256
	osdOwnerSize      = 256
	osdExtendedSize   = 4096
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
	mapHandle windows.Handle
	view      []byte
	entry     int
}

func newSharedMemorySink() osdSink { return &sharedMemorySink{entry: -1} }

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
	text := fmt.Sprintf("Cooler Fan: %d RPM", rpm)
	if layout.version >= 0x00020007 && layout.entrySize >= osdExtendedOffset+osdExtendedSize {
		writeCString(entry[osdExtendedOffset:], osdExtendedSize, text)
	} else {
		writeCString(entry[osdTextOffset:], osdTextSize, text)
	}
	atomic.AddUint32((*uint32)(unsafe.Pointer(&s.view[osdFrameOffset])), 1)
	return true
}

func (s *sharedMemorySink) findEntry(layout sharedMemoryLayout) int {
	for pass := 0; pass < 2; pass++ {
		for i := 1; i < layout.arraySize; i++ {
			if s.ownedEntry(layout, i) {
				return i
			}
			entry := s.entryBytes(layout, i)
			if pass == 1 && readCString(entry[osdOwnerOffset:osdOwnerOffset+osdOwnerSize]) == "" {
				writeCString(entry[osdOwnerOffset:], osdOwnerSize, ownerName)
				return i
			}
		}
	}
	return -1
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
	if layout.version < 0x0002000e {
		return nil, true
	}
	busy := (*uint32)(unsafe.Pointer(&s.view[busyOffset]))
	return busy, tryLock(busy)
}

func decodeLayout(view []byte, maxSize int) (sharedMemoryLayout, bool) {
	if len(view) < headerSize || binary.LittleEndian.Uint32(view[:4]) != rtssSignature {
		return sharedMemoryLayout{}, false
	}
	version := binary.LittleEndian.Uint32(view[4:8])
	entrySize := int(binary.LittleEndian.Uint32(view[20:24]))
	arrayOffset := int(binary.LittleEndian.Uint32(view[24:28]))
	arraySize := int(binary.LittleEndian.Uint32(view[28:32]))
	if version < 0x00020000 || entrySize < osdOwnerOffset+osdOwnerSize || arrayOffset < headerSize || arraySize <= 1 {
		return sharedMemoryLayout{}, false
	}
	if entrySize > (maxSize-arrayOffset)/arraySize {
		return sharedMemoryLayout{}, false
	}
	requiredSize := arrayOffset + entrySize*arraySize
	if requiredSize > maxSize {
		return sharedMemoryLayout{}, false
	}
	return sharedMemoryLayout{
		version:      version,
		entrySize:    entrySize,
		arrayOffset:  arrayOffset,
		arraySize:    arraySize,
		requiredSize: requiredSize,
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

	header := unsafe.Slice((*byte)(unsafe.Pointer(view)), headerSize)
	layout, ok := decodeLayout(header, maxViewSize)
	if !ok {
		windows.UnmapViewOfFile(view)
		windows.CloseHandle(h)
		return false
	}
	s.mapHandle = h
	s.view = unsafe.Slice((*byte)(unsafe.Pointer(view)), layout.requiredSize)
	return true
}

func (s *sharedMemorySink) unmap() {
	if len(s.view) > 0 {
		windows.UnmapViewOfFile(uintptr(unsafe.Pointer(&s.view[0])))
		s.view = nil
	}
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

func (s *sharedMemorySink) releaseEntry(layout sharedMemoryLayout) {
	busy, ok := s.tryLock(layout)
	if !ok {
		return
	}
	if busy != nil {
		defer unlock(busy)
	}
	if s.entry > 0 && s.entry < layout.arraySize && s.ownedEntry(layout, s.entry) {
		clear(s.entryBytes(layout, s.entry))
		atomic.AddUint32((*uint32)(unsafe.Pointer(&s.view[osdFrameOffset])), 1)
	}
}
