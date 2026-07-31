package config

import (
	"encoding/json"
	"testing"

	"github.com/TIANLI0/THRM/internal/types"
)

func TestApplyMissingRTSSDefaults(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want types.RTSSConfig
	}{
		{
			name: "missing config",
			raw:  `{}`,
			want: types.GetDefaultRTSSConfig(),
		},
		{
			name: "missing interval",
			raw:  `{"rtss":{"enabled":true}}`,
			want: types.RTSSConfig{Enabled: true, UpdateIntervalMS: 500},
		},
		{
			name: "valid explicit values",
			raw:  `{"rtss":{"enabled":false,"updateIntervalMs":250}}`,
			want: types.RTSSConfig{Enabled: false, UpdateIntervalMS: 250},
		},
		{
			name: "invalid interval",
			raw:  `{"rtss":{"enabled":true,"updateIntervalMs":750}}`,
			want: types.RTSSConfig{Enabled: true, UpdateIntervalMS: 500},
		},
		{
			name: "null config",
			raw:  `{"rtss":null}`,
			want: types.GetDefaultRTSSConfig(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var cfg types.AppConfig
			if err := json.Unmarshal([]byte(test.raw), &cfg); err != nil {
				t.Fatalf("unmarshal config: %v", err)
			}
			var rawConfig map[string]json.RawMessage
			if err := json.Unmarshal([]byte(test.raw), &rawConfig); err != nil {
				t.Fatalf("unmarshal raw config: %v", err)
			}

			applyMissingRTSSDefaults(&cfg, rawConfig)
			if cfg.RTSS != test.want {
				t.Fatalf("RTSS config = %+v, want %+v", cfg.RTSS, test.want)
			}
		})
	}
}
