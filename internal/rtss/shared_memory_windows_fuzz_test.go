//go:build windows

package rtss

import (
	"encoding/binary"
	"testing"
)

func FuzzDecodeLayout(f *testing.F) {
	const (
		validEntrySize   = osdExtendedOffset + osdExtendedSize
		validArrayOffset = 96
		validArraySize   = 8
		validMappedSize  = validArrayOffset + validEntrySize*validArraySize
	)

	valid := testHeader(validEntrySize, validArrayOffset, validArraySize)
	f.Add(valid, uint32(validMappedSize))
	f.Add(valid, uint32(maxViewSize))
	f.Add([]byte{}, uint32(0))
	f.Add(make([]byte, headerSize-1), uint32(headerSize-1))
	f.Add(testHeader(^uint32(0), ^uint32(0), ^uint32(0)), uint32(maxViewSize))

	f.Fuzz(func(t *testing.T, header []byte, rawMappedSize uint32) {
		// mappedViewSize caps the production input at maxViewSize. Keeping the
		// fuzz domain identical also makes int conversion safe on Windows 386.
		mappedSize := int(rawMappedSize % uint32(maxViewSize+1))
		layout, ok := decodeLayout(header, mappedSize)
		if !ok {
			return
		}

		if len(header) < headerSize {
			t.Fatalf("accepted a %d-byte header", len(header))
		}
		if mappedSize < headerSize {
			t.Fatalf("accepted mapped size %d smaller than the header", mappedSize)
		}

		signature := binary.LittleEndian.Uint32(header[0:4])
		version := binary.LittleEndian.Uint32(header[4:8])
		entrySize := binary.LittleEndian.Uint32(header[20:24])
		arrayOffset := binary.LittleEndian.Uint32(header[24:28])
		arraySize := binary.LittleEndian.Uint32(header[28:32])

		if signature != rtssSignature {
			t.Fatalf("accepted signature %#x", signature)
		}
		if version < 0x00020000 || layout.version != version {
			t.Fatalf("accepted or decoded invalid version %#x as %#x", version, layout.version)
		}
		if entrySize < osdOwnerOffset+osdOwnerSize || layout.entrySize != int(entrySize) {
			t.Fatalf("accepted or decoded invalid entry size %d as %d", entrySize, layout.entrySize)
		}
		if arrayOffset < headerSize || layout.arrayOffset != int(arrayOffset) {
			t.Fatalf("accepted or decoded invalid array offset %d as %d", arrayOffset, layout.arrayOffset)
		}
		if arraySize <= 1 || layout.arraySize != int(arraySize) {
			t.Fatalf("accepted or decoded invalid array size %d as %d", arraySize, layout.arraySize)
		}

		requiredSize := uint64(arrayOffset) + uint64(entrySize)*uint64(arraySize)
		if requiredSize > uint64(mappedSize) {
			t.Fatalf("accepted required size %d beyond mapped size %d", requiredSize, mappedSize)
		}
		if layout.requiredSize != int(requiredSize) {
			t.Fatalf("required size = %d, want %d", layout.requiredSize, requiredSize)
		}

		lastEntryStart := uint64(layout.arrayOffset) + uint64(layout.entrySize)*uint64(layout.arraySize-1)
		lastEntryEnd := lastEntryStart + uint64(layout.entrySize)
		if lastEntryEnd != uint64(layout.requiredSize) {
			t.Fatalf("last entry ends at %d, layout ends at %d", lastEntryEnd, layout.requiredSize)
		}
	})
}
