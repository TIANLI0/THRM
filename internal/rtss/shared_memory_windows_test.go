//go:build windows

package rtss

import (
	"encoding/binary"
	"testing"

	"golang.org/x/sys/windows"
)

func testHeader(entrySize, arrayOffset, arraySize uint32) []byte {
	return testHeaderVersion(0x00020015, entrySize, arrayOffset, arraySize)
}

func testHeaderVersion(version, entrySize, arrayOffset, arraySize uint32) []byte {
	header := make([]byte, headerSize)
	binary.LittleEndian.PutUint32(header[0:4], rtssSignature)
	binary.LittleEndian.PutUint32(header[4:8], version)
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
	mapped, ok := mappedViewBytes(view, mappingSize, got)
	if !ok {
		t.Fatal("verified mapped view could not be converted to bytes")
	}
	mapped[0] = 0x5a
	if mapped[0] != 0x5a {
		t.Fatal("mapped view byte was not writable")
	}
	if _, ok := mappedViewBytes(view, got+1, got); ok {
		t.Fatal("slice larger than the verified mapped region was accepted")
	}
	if _, ok := mappedViewBytes(0, headerSize, got); ok {
		t.Fatal("null mapped view was converted to bytes")
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

func TestFormatOSDTextResetsInheritedStyles(t *testing.T) {
	if got, want := formatOSDText(1500), "<C><S>Cooler Fan: 1500 RPM"; got != want {
		t.Fatalf("formatOSDText = %q, want %q", got, want)
	}
	if got, want := formatOSDTextAt(1500, "custom", -3, 7), "<P=-3,7><C><S>Cooler Fan: 1500 RPM"; got != want {
		t.Fatalf("custom formatOSDText = %q, want %q", got, want)
	}
}

func TestExecutableDirectory(t *testing.T) {
	got := executableDirectory(`"D:\app\RivaTuner Statistics Server\uninstall.exe" /S`)
	if want := `D:\app\RivaTuner Statistics Server`; got != want {
		t.Fatalf("executableDirectory = %q, want %q", got, want)
	}
}

func TestWriteOSDTextUsesVersionSpecificBuffer(t *testing.T) {
	tests := []struct {
		name    string
		version uint32
		size    int
		offset  int
		length  int
	}{
		{name: "v2.6 legacy", version: 0x00020006, size: osdExtended2Offset + osdExtended2Size, offset: osdTextOffset, length: osdTextSize},
		{name: "v2.7 extended", version: rtssVersionExtended, size: osdExtended2Offset + osdExtended2Size, offset: osdExtendedOffset, length: osdExtendedSize},
		{name: "v2.20 extended2", version: rtssVersionExtended2, size: osdExtended2Offset + osdExtended2Size, offset: osdExtended2Offset, length: osdExtended2Size},
		{name: "v2.20 small entry fallback", version: rtssVersionExtended2, size: osdExtendedOffset + osdExtendedSize, offset: osdExtendedOffset, length: osdExtendedSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := make([]byte, tt.size)
			writeOSDText(entry, tt.version, "Cooler Fan: 1500 RPM")
			if got := readCString(entry[tt.offset : tt.offset+tt.length]); got != "Cooler Fan: 1500 RPM" {
				t.Fatalf("OSD text = %q", got)
			}
		})
	}
}

func newEntrySelectionTestSink(t *testing.T, arraySize int) (*sharedMemorySink, sharedMemoryLayout) {
	t.Helper()
	const (
		entrySize   = osdExtendedOffset + osdExtendedSize
		arrayOffset = 96
	)
	view := make([]byte, arrayOffset+entrySize*arraySize)
	copy(view, testHeader(entrySize, arrayOffset, uint32(arraySize)))
	layout, ok := decodeLayout(view, len(view))
	if !ok {
		t.Fatal("test layout was rejected")
	}
	return &sharedMemorySink{view: view, entry: -1}, layout
}

func setTestEntryOwner(sink *sharedMemorySink, layout sharedMemoryLayout, index int, owner string) {
	writeCString(sink.entryBytes(layout, index)[osdOwnerOffset:], osdOwnerSize, owner)
}

func testEntryOwner(sink *sharedMemorySink, layout sharedMemoryLayout, index int) string {
	entry := sink.entryBytes(layout, index)
	return readCString(entry[osdOwnerOffset : osdOwnerOffset+osdOwnerSize])
}

func TestSharedMemorySinkSelectsHighestUsableSlot(t *testing.T) {
	tests := []struct {
		name   string
		owners map[int]string
		want   int
	}{
		{name: "all empty", want: 7},
		{name: "overlay editor in low slot", owners: map[int]string{2: "RTSSOverlayEditor"}, want: 7},
		{name: "highest occupied", owners: map[int]string{7: "HWiNFO"}, want: 6},
		{name: "reuse highest THRM slot", owners: map[int]string{7: ownerName}, want: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink, layout := newEntrySelectionTestSink(t, 8)
			setTestEntryOwner(sink, layout, 0, "MSIAfterburner")
			for index, owner := range tt.owners {
				setTestEntryOwner(sink, layout, index, owner)
			}

			if got := sink.findEntry(layout); got != tt.want {
				t.Fatalf("claimed entry = %d, want %d", got, tt.want)
			}
			if got := testEntryOwner(sink, layout, tt.want); got != ownerName {
				t.Fatalf("claimed entry owner = %q, want %q", got, ownerName)
			}
			if got := testEntryOwner(sink, layout, 0); got != "MSIAfterburner" {
				t.Fatalf("primary entry owner = %q", got)
			}
		})
	}
}

func TestSharedMemorySinkReusesOnlyAvailableOwnedSlot(t *testing.T) {
	sink, layout := newEntrySelectionTestSink(t, 8)
	for i := 1; i < layout.arraySize; i++ {
		setTestEntryOwner(sink, layout, i, "foreign")
	}
	setTestEntryOwner(sink, layout, 2, ownerName)

	if got := sink.findEntry(layout); got != 2 {
		t.Fatalf("claimed entry = %d, want 2", got)
	}
}

func TestSharedMemorySinkCleansDuplicateOwnedSlots(t *testing.T) {
	sink, layout := newEntrySelectionTestSink(t, 8)
	setTestEntryOwner(sink, layout, 2, ownerName)
	setTestEntryOwner(sink, layout, 6, ownerName)
	setTestEntryOwner(sink, layout, 7, "HWiNFO")

	if got := sink.findEntry(layout); got != 6 {
		t.Fatalf("claimed entry = %d, want 6", got)
	}
	if got := testEntryOwner(sink, layout, 2); got != "" {
		t.Fatalf("duplicate entry owner = %q, want empty", got)
	}
	if got := testEntryOwner(sink, layout, 6); got != ownerName {
		t.Fatalf("selected entry owner = %q, want %q", got, ownerName)
	}
}

func TestSharedMemorySinkDoesNotClaimForeignOrPrimarySlots(t *testing.T) {
	sink, layout := newEntrySelectionTestSink(t, 8)
	setTestEntryOwner(sink, layout, 0, ownerName)
	for i := 1; i < layout.arraySize; i++ {
		setTestEntryOwner(sink, layout, i, "foreign")
	}

	if got := sink.findEntry(layout); got != -1 {
		t.Fatalf("claimed entry = %d, want -1", got)
	}
	if got := testEntryOwner(sink, layout, 0); got != ownerName {
		t.Fatalf("primary entry owner = %q, want %q", got, ownerName)
	}
}

func TestSharedMemorySinkOwnsAndReleasesHighestNonPrimarySlot(t *testing.T) {
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
	if entry := sink.findEntry(layout); entry != 3 {
		t.Fatalf("claimed entry = %d, want 3", entry)
	}
	if got := readCString(sink.entryBytes(layout, 0)[osdOwnerOffset:]); got != "MSIAfterburner" {
		t.Fatalf("primary entry owner = %q", got)
	}
	if got := readCString(sink.entryBytes(layout, 3)[osdOwnerOffset:]); got != ownerName {
		t.Fatalf("claimed entry owner = %q, want %q", got, ownerName)
	}

	sink.entry = 3
	entry := sink.entryBytes(layout, 3)
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

func TestSharedMemorySinkClearsUnownedSlotBeforeClaiming(t *testing.T) {
	const (
		entrySize   = osdExtendedOffset + osdExtendedSize
		arrayOffset = 96
		arraySize   = 3
	)
	view := make([]byte, arrayOffset+entrySize*arraySize)
	copy(view, testHeader(entrySize, arrayOffset, arraySize))
	layout, ok := decodeLayout(view, len(view))
	if !ok {
		t.Fatal("test layout was rejected")
	}

	sink := &sharedMemorySink{view: view, entry: -1}
	writeCString(sink.entryBytes(layout, 0)[osdOwnerOffset:], osdOwnerSize, "MSIAfterburner")
	entry := sink.entryBytes(layout, 2)
	for i := range entry {
		entry[i] = 0xa5
	}
	clear(entry[osdOwnerOffset : osdOwnerOffset+osdOwnerSize])

	if claimed := sink.findEntry(layout); claimed != 2 {
		t.Fatalf("claimed entry = %d, want 2", claimed)
	}
	if got := readCString(entry[osdOwnerOffset : osdOwnerOffset+osdOwnerSize]); got != ownerName {
		t.Fatalf("claimed entry owner = %q, want %q", got, ownerName)
	}
	for i, value := range entry {
		insideOwner := i >= osdOwnerOffset && i < osdOwnerOffset+len(ownerName)
		if !insideOwner && value != 0 {
			t.Fatalf("claimed entry byte %d = %#x, want 0", i, value)
		}
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

func TestSharedMemorySinkMigratesOwnedSlotToHighestAvailable(t *testing.T) {
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
	if entry := sink.findEntry(layout); entry != 3 {
		t.Fatalf("migrated entry = %d, want 3", entry)
	}
	if got := readCString(sink.entryBytes(layout, 2)[osdOwnerOffset:]); got != "" {
		t.Fatalf("old entry owner = %q, want empty", got)
	}
}
