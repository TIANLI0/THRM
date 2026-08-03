//go:build linux

package guiapp

import (
	"bytes"
	"testing"
)

func TestCappedBufferLimitsStoredDataWithoutShortWrite(t *testing.T) {
	buffer := cappedBuffer{limit: 5}
	if written, err := buffer.Write([]byte("1234")); err != nil || written != 4 {
		t.Fatalf("first Write() = (%d, %v), want (4, nil)", written, err)
	}
	if written, err := buffer.Write([]byte("56789")); err != nil || written != 5 {
		t.Fatalf("second Write() = (%d, %v), want (5, nil)", written, err)
	}
	if !bytes.Equal(buffer.buffer.Bytes(), []byte("12345")) {
		t.Fatalf("stored data = %q, want %q", buffer.buffer.Bytes(), "12345")
	}
}
