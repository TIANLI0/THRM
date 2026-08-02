//go:build windows

package rtss

import (
	"encoding/binary"
	"testing"

	"golang.org/x/sys/windows"
)

func testHeader(entrySize, arrayOffset, arraySize uint32) []byte {
	header := make([]byte, headerSize)
	binary.LittleEndian.PutUint32(header[0:4], rtssSignature)
	binary.LittleEndian.PutUint32(header[4:8], 0x00020015)
	binary.LittleEndian.PutUint32(header[20:24], entrySize)
	binary.LittleEndian.PutUint32(header[24:28], arrayOffset)
	binary.LittleEndian.PutUint32(header[28:32], arraySize)
	return header
}

func TestDecodeLayout(t *testing.T) {
	header := testHeader(299520, 96, 8)
	layout, ok := decodeLayout(header, maxViewSize)
	if !ok {
		t.Fatal("valid RTSS header was rejected")
	}
	if layout.version != 0x00020015 || layout.entrySize != 299520 || layout.arrayOffset != 96 || layout.arraySize != 8 {
		t.Fatalf("unexpected layout: %+v", layout)
	}
	if layout.requiredSize != 96+299520*8 {
		t.Fatalf("required size = %d", layout.requiredSize)
	}
}

func TestDecodeLayoutRejectsInvalidHeaders(t *testing.T) {
	tests := map[string][]byte{
		"short":         make([]byte, headerSize-1),
		"bad signature": testHeader(299520, 96, 8),
		"small entry":   testHeader(128, 96, 8),
		"bad offset":    testHeader(299520, 4, 8),
		"one slot":      testHeader(299520, 96, 1),
	}
	tests["bad signature"][0] = 0
	for name, header := range tests {
		if _, ok := decodeLayout(header, maxViewSize); ok {
			t.Fatalf("%s header was accepted", name)
		}
	}
}

func TestDecodeLayoutRejectsDeclaredSizeBeyondView(t *testing.T) {
	header := testHeader(4096, 96, 4)
	requiredSize := 96 + 4096*4
	if _, ok := decodeLayout(header, requiredSize-1); ok {
		t.Fatal("layout larger than the mapped view was accepted")
	}
	if _, ok := decodeLayout(header, requiredSize); !ok {
		t.Fatal("layout matching the mapped view was rejected")
	}
}

func TestMappedViewSize(t *testing.T) {
	const mappingSize = 64 << 10
	mapping, err := windows.CreateFileMapping(windows.InvalidHandle, nil, windows.PAGE_READWRITE, 0, mappingSize, nil)
	if err != nil {
		t.Fatalf("CreateFileMapping: %v", err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(mapping) })

	view, err := windows.MapViewOfFile(mapping, mapReadWrite, 0, 0, 0)
	if err != nil {
		t.Fatalf("MapViewOfFile: %v", err)
	}
	t.Cleanup(func() { _ = windows.UnmapViewOfFile(view) })

	got, ok := mappedViewSize(view)
	if !ok {
		t.Fatal("valid mapped view was rejected")
	}
	if got < mappingSize || got > maxViewSize {
		t.Fatalf("mapped view size = %d, want at least %d and at most %d", got, mappingSize, maxViewSize)
	}
	if _, ok := mappedViewSize(0); ok {
		t.Fatal("null mapped view was accepted")
	}
}

func TestCStringHelpers(t *testing.T) {
	buffer := make([]byte, 16)
	writeCString(buffer, len(buffer), "Cooler Fan")
	if got := readCString(buffer); got != "Cooler Fan" {
		t.Fatalf("readCString = %q", got)
	}
	writeCString(buffer, len(buffer), "12345678901234567890")
	if got := readCString(buffer); got != "123456789012345" {
		t.Fatalf("truncated string = %q", got)
	}
}

func TestSharedMemorySinkOwnsAndReleasesNonPrimarySlot(t *testing.T) {
	const (
		entrySize   = osdExtendedOffset + osdExtendedSize
		arrayOffset = 96
		arraySize   = 4
	)
	view := make([]byte, arrayOffset+entrySize*arraySize)
	copy(view, testHeader(entrySize, arrayOffset, arraySize))
	layout, ok := decodeLayout(view, len(view))
	if !ok {
		t.Fatal("test layout was rejected")
	}

	sink := &sharedMemorySink{view: view, entry: -1}
	writeCString(sink.entryBytes(layout, 0)[osdOwnerOffset:], osdOwnerSize, "MSIAfterburner")
	if entry := sink.findEntry(layout); entry != 1 {
		t.Fatalf("claimed entry = %d, want 1", entry)
	}
	if got := readCString(sink.entryBytes(layout, 0)[osdOwnerOffset:]); got != "MSIAfterburner" {
		t.Fatalf("primary entry owner = %q", got)
	}
	if got := readCString(sink.entryBytes(layout, 1)[osdOwnerOffset:]); got != ownerName {
		t.Fatalf("claimed entry owner = %q, want %q", got, ownerName)
	}

	sink.entry = 1
	entry := sink.entryBytes(layout, 1)
	writeCString(entry[osdExtendedOffset:], osdExtendedSize, "Cooler Fan: 1500 RPM")
	sink.releaseEntry(layout)
	if got := readCString(entry[osdOwnerOffset : osdOwnerOffset+osdOwnerSize]); got != "" {
		t.Fatalf("released entry owner = %q", got)
	}
	if got := readCString(entry[osdExtendedOffset : osdExtendedOffset+osdExtendedSize]); got != "" {
		t.Fatalf("released entry text = %q", got)
	}
	if got := binary.LittleEndian.Uint32(view[osdFrameOffset:]); got != 1 {
		t.Fatalf("OSD frame = %d, want 1", got)
	}
}

func ownedEntryTestSink(t *testing.T) (*sharedMemorySink, sharedMemoryLayout, []byte) {
	t.Helper()
	const (
		entrySize   = osdExtendedOffset + osdExtendedSize
		arrayOffset = 96
		arraySize   = 4
	)
	view := make([]byte, arrayOffset+entrySize*arraySize)
	copy(view, testHeader(entrySize, arrayOffset, arraySize))
	layout, ok := decodeLayout(view, len(view))
	if !ok {
		t.Fatal("test layout was rejected")
	}

	sink := &sharedMemorySink{view: view, entry: 1}
	entry := sink.entryBytes(layout, 1)
	writeCString(entry[osdOwnerOffset:], osdOwnerSize, ownerName)
	writeCString(entry[osdExtendedOffset:], osdExtendedSize, "Cooler Fan: 1500 RPM")
	return sink, layout, entry
}

func TestSharedMemorySinkReleaseRetriesBusyLock(t *testing.T) {
	sink, layout, entry := ownedEntryTestSink(t)
	view := sink.view
	binary.LittleEndian.PutUint32(view[busyOffset:], 1)

	waits := 0
	if !sink.releaseEntryWithWait(layout, func() {
		waits++
		binary.LittleEndian.PutUint32(view[busyOffset:], 0)
	}) {
		t.Fatal("release did not acquire the lock after it became available")
	}
	if waits != 1 {
		t.Fatalf("release waits = %d, want 1", waits)
	}
	if got := readCString(entry[osdOwnerOffset : osdOwnerOffset+osdOwnerSize]); got != "" {
		t.Fatalf("released entry owner = %q", got)
	}
	if got := readCString(entry[osdExtendedOffset : osdExtendedOffset+osdExtendedSize]); got != "" {
		t.Fatalf("released entry text = %q", got)
	}
	if got := binary.LittleEndian.Uint32(view[osdFrameOffset:]); got != 1 {
		t.Fatalf("OSD frame = %d, want 1", got)
	}
}

func TestSharedMemorySinkReleaseRetryIsBounded(t *testing.T) {
	sink, layout, entry := ownedEntryTestSink(t)
	view := sink.view
	binary.LittleEndian.PutUint32(view[busyOffset:], 1)

	waits := 0
	if sink.releaseEntryWithWait(layout, func() { waits++ }) {
		t.Fatal("release unexpectedly acquired a permanently busy lock")
	}
	if waits != releaseLockAttempts-1 {
		t.Fatalf("release waits = %d, want %d", waits, releaseLockAttempts-1)
	}
	if got := readCString(entry[osdOwnerOffset : osdOwnerOffset+osdOwnerSize]); got != ownerName {
		t.Fatalf("busy entry owner = %q, want %q", got, ownerName)
	}
	if got := readCString(entry[osdExtendedOffset : osdExtendedOffset+osdExtendedSize]); got != "Cooler Fan: 1500 RPM" {
		t.Fatalf("busy entry text = %q", got)
	}
	if got := binary.LittleEndian.Uint32(view[osdFrameOffset:]); got != 0 {
		t.Fatalf("OSD frame = %d, want 0", got)
	}
}

func TestSharedMemorySinkReusesOwnedSlot(t *testing.T) {
	const (
		entrySize   = osdExtendedOffset + osdExtendedSize
		arrayOffset = 96
		arraySize   = 4
	)
	view := make([]byte, arrayOffset+entrySize*arraySize)
	copy(view, testHeader(entrySize, arrayOffset, arraySize))
	layout, ok := decodeLayout(view, len(view))
	if !ok {
		t.Fatal("test layout was rejected")
	}

	sink := &sharedMemorySink{view: view, entry: -1}
	writeCString(sink.entryBytes(layout, 2)[osdOwnerOffset:], osdOwnerSize, ownerName)
	if entry := sink.findEntry(layout); entry != 2 {
		t.Fatalf("reused entry = %d, want 2", entry)
	}
}
