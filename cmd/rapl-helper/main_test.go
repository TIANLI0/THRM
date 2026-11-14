//go:build linux

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadEnergyUJ(t *testing.T) {
	path := filepath.Join(t.TempDir(), "energy_uj")
	if err := os.WriteFile(path, []byte("123456789\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	value, err := readEnergyUJ(file)
	if err != nil {
		t.Fatal(err)
	}
	if value != 123456789 {
		t.Fatalf("energy = %d", value)
	}
}

func TestEnergyDeltaWatts(t *testing.T) {
	if got := energyDeltaWatts(1_000_000, 11_000_000, time.Second); got != 10 {
		t.Fatalf("watts = %f, want 10", got)
	}
	if got := energyDeltaWatts(10, 1, time.Second); got != 0 {
		t.Fatalf("wrapped watts = %f, want 0", got)
	}
	if got := energyDeltaWatts(1, 2, 0); got != 0 {
		t.Fatalf("zero elapsed watts = %f, want 0", got)
	}
}
