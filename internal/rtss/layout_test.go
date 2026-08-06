package rtss

import (
	"strings"
	"testing"
)

func TestInspectOverlayLayoutRecognizesAnchorAndCandidates(t *testing.T) {
	data := []byte("[General]\nLayers=3\n[Layer0]\nName=CPU\nText=CPU Temp\n[Layer1]\nName=\nText=\nSize=1\n[Layer2]\nName=THRM Anchor\nText=\nSize=1\n")
	state, index, count := inspectOverlayLayout(data)
	if state != anchorStateConfirmed || index != 2 || count != 3 {
		t.Fatalf("anchor status = %q, index=%d, count=%d", state, index, count)
	}
}

func TestInspectOverlayLayoutDoesNotTreatEveryOnePercentLayerAsAnchor(t *testing.T) {
	data := []byte("[General]\nLayers=2\n[Layer0]\nName=Spacer\nText=\nSize=1\n[Layer1]\nName=CPU\nText=CPU Temp\n")
	state, index, _ := inspectOverlayLayout(data)
	if state != anchorStateNeedsLast || index != 0 {
		t.Fatalf("candidate status = %q, index=%d", state, index)
	}
}

func TestAppendAnchorIsIdempotenceHandledByInspection(t *testing.T) {
	data := []byte("[Master]\r\nImplementation=2\r\n[Settings]\r\nName=\r\n[General]\r\nLayers=1\r\n[Source0]\r\nName=CPU\r\n[Layer0]\r\nName=CPU\r\nText=CPU Temp\r\n")
	updated, err := appendAnchor(data)
	if err != nil {
		t.Fatalf("appendAnchor failed: %v", err)
	}
	if got := string(updated); got == string(data) {
		t.Fatal("appendAnchor did not change layout")
	}
	if strings.Contains(string(updated), "\r\r\n") {
		t.Fatal("appendAnchor introduced malformed CRLF line endings")
	}
	if !strings.Contains(string(updated), "[Source0]\r\nName=CPU\r\n") {
		t.Fatal("appendAnchor did not preserve non-layer sections")
	}
	if !strings.Contains(string(updated), "PositionX=0\r\nPositionY=-1\r\n") {
		t.Fatal("new anchor was not placed below the existing baseline")
	}
	state, index, count := inspectOverlayLayout(updated)
	if state != anchorStateConfirmed || index != 1 || count != 2 {
		t.Fatalf("updated anchor status = %q, index=%d, count=%d", state, index, count)
	}
}

func TestDefaultAnchorPositionUsesCommonColumnAndLowestBaseline(t *testing.T) {
	layers := []overlayLayer{
		{text: "CPU", positionX: 0, positionY: -8},
		{text: "GPU", positionX: 0, positionY: -4},
		{text: "Diagnostic", positionX: -20, positionY: 0},
	}
	if x, y := defaultAnchorPosition(layers); x != 0 || y != -9 {
		t.Fatalf("default anchor position = (%d,%d), want (0,-9)", x, y)
	}
}

func TestConfigureAnchorMarksAnonymousCandidate(t *testing.T) {
	data := []byte("[General]\r\nLayers=2\r\n[Layer0]\r\nName=CPU\r\nText=CPU Temp\r\n[Layer1]\r\nName=\r\nText=\r\nSize=1\r\n")
	updated, err := configureAnchor(data)
	if err != nil {
		t.Fatalf("configureAnchor failed: %v", err)
	}
	if !strings.Contains(string(updated), "Name=THRM Anchor\r\n") {
		t.Fatalf("candidate was not marked: %q", updated)
	}
	state, index, count := inspectOverlayLayout(updated)
	if state != anchorStateConfirmed || index != 1 || count != 2 {
		t.Fatalf("configured status = %q, index=%d, count=%d", state, index, count)
	}
}

func TestConfigureAnchorMovesKnownLayerToEnd(t *testing.T) {
	data := []byte("[General]\nLayers=3\n[Layer0]\nName=CPU\nText=CPU Temp\n[Layer1]\nName=THRM Anchor\nText=\nSize=1\n[Layer2]\nName=GPU\nText=GPU Temp\n")
	updated, err := configureAnchor(data)
	if err != nil {
		t.Fatalf("configureAnchor failed: %v", err)
	}
	if strings.Index(string(updated), "Name=THRM Anchor") < strings.Index(string(updated), "Name=GPU") {
		t.Fatalf("anchor was not moved after the last layer: %q", updated)
	}
	state, index, count := inspectOverlayLayout(updated)
	if state != anchorStateConfirmed || index != 2 || count != 3 {
		t.Fatalf("moved status = %q, index=%d, count=%d", state, index, count)
	}
}

func TestConfigureAnchorRejectsAmbiguousCandidates(t *testing.T) {
	data := []byte("[General]\nLayers=2\n[Layer0]\nText=\nSize=1\n[Layer1]\nText=\nSize=1\n")
	if _, err := configureAnchor(data); err == nil {
		t.Fatal("multiple anonymous candidates were accepted")
	}
}

func TestConfigureAnchorRejectsInterleavedSections(t *testing.T) {
	data := []byte("[General]\nLayers=2\n[Layer0]\nText=\nSize=1\n[Metadata]\nValue=keep\n[Layer1]\nText=CPU Temp\n")
	if _, err := configureAnchor(data); err == nil {
		t.Fatal("layout with an interleaved non-layer section was accepted")
	}
}
