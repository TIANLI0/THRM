package coreapp

import (
	"testing"

	"github.com/TIANLI0/THRM/internal/config"
	"github.com/TIANLI0/THRM/internal/types"
)

func TestNormalizeUpdatedRTSSConfig(t *testing.T) {
	previous := types.RTSSConfig{Enabled: true, UpdateIntervalMS: 1000, PositionMode: types.RTSSPositionModeAnchor}
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
			want: types.RTSSConfig{Enabled: false, UpdateIntervalMS: 250, PositionMode: types.RTSSPositionModeAnchor},
		},
		{
			name: "normalizes invalid update",
			next: types.RTSSConfig{Enabled: true, UpdateIntervalMS: 750},
			want: types.RTSSConfig{Enabled: true, UpdateIntervalMS: 1000, PositionMode: types.RTSSPositionModeAnchor},
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

func TestPreviewRTSSPositionDoesNotPersistConfig(t *testing.T) {
	app := &CoreApp{configManager: config.NewManager(t.TempDir(), nil)}
	before := app.configManager.Get().RTSS

	app.PreviewRTSSPosition(types.RTSSPositionModeCustom, 48, -24)

	if after := app.configManager.Get().RTSS; after != before {
		t.Fatalf("preview changed persisted config: before=%+v after=%+v", before, after)
	}
}
