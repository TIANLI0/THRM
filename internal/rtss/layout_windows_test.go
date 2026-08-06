//go:build windows

package rtss

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteLayoutAtomicallyPreservesOriginalWhenReplaceFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "layout.ovl")
	original := []byte("original layout")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("replace failed")
	err := writeLayoutAtomicallyWithReplace(path, []byte("updated layout"), func(string, string) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("writeLayoutAtomicallyWithReplace() error = %v, want %v", err, wantErr)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read original layout: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("original layout = %q, want %q", got, original)
	}
}

func TestWriteLayoutAtomicallyReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "layout.ovl")
	if err := os.WriteFile(path, []byte("original layout"), 0644); err != nil {
		t.Fatal(err)
	}

	want := []byte("updated layout")
	if err := writeLayoutAtomically(path, want); err != nil {
		t.Fatalf("writeLayoutAtomically() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("updated layout = %q, want %q", got, want)
	}
}
