package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestNormalizeUpdatedRTSSConfig(t *testing.T) {
	previous := types.RTSSConfig{Enabled: true, UpdateIntervalMS: 1000}
	tests := []struct {
		name string
		next types.RTSSConfig
		want types.RTSSConfig
	}{
		{
			name: "preserves config omitted by older client",
			next: types.RTSSConfig{},
			want: previous,
		},
		{
			name: "accepts supported update",
			next: types.RTSSConfig{Enabled: false, UpdateIntervalMS: 250},
			want: types.RTSSConfig{Enabled: false, UpdateIntervalMS: 250},
		},
		{
			name: "normalizes invalid update",
			next: types.RTSSConfig{Enabled: true, UpdateIntervalMS: 750},
			want: types.RTSSConfig{Enabled: true, UpdateIntervalMS: 500},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeUpdatedRTSSConfig(test.next, previous); got != test.want {
				t.Fatalf("normalized config = %+v, want %+v", got, test.want)
			}
		})
	}
}
