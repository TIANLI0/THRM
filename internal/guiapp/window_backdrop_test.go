package guiapp

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

func TestBackdropTypeForMode(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		windows11 bool
		want      windows.BackdropType
	}{
		{name: "auto on Windows 11", mode: "auto", windows11: true, want: windows.Mica},
		{name: "auto on Windows 10", mode: "auto", windows11: false, want: windows.None},
		{name: "legacy on", mode: "on", windows11: true, want: windows.Mica},
		{name: "acrylic", mode: "acrylic", windows11: true, want: windows.Acrylic},
		{name: "mica", mode: "mica", windows11: true, want: windows.Mica},
		{name: "mica alt", mode: "tabbed", windows11: true, want: windows.Tabbed},
		{name: "off", mode: "off", windows11: true, want: windows.None},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := backdropTypeForMode(tt.mode, tt.windows11); got != tt.want {
				t.Fatalf("backdropTypeForMode(%q, %v) = %v, want %v", tt.mode, tt.windows11, got, tt.want)
			}
		})
	}
}
