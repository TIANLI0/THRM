//go:build windows

package rtss

import (
	"encoding/binary"
	"testing"
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
	writeCString(sink.entryBytes(layout, 1)[osdExtendedOffset:], osdExtendedSize, "Cooler Fan: 1500 RPM")
	sink.releaseEntry(layout)
	if got := readCString(sink.entryBytes(layout, 1)); got != "" {
		t.Fatalf("released entry still contains %q", got)
	}
	if got := binary.LittleEndian.Uint32(view[osdFrameOffset:]); got != 1 {
		t.Fatalf("OSD frame = %d, want 1", got)
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
