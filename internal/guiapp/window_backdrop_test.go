package guiapp

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestBackdropTypeForMode(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		windows11 bool
		highDPI   bool
		want      application.BackdropType
	}{
		{name: "auto on high-DPI Windows 11", mode: "auto", windows11: true, highDPI: true, want: application.Mica},
		// 1080p@100% 这类低缩放屏幕：透明背景会让文字失去 ClearType 而发虚。
		{name: "auto on low-DPI Windows 11", mode: "auto", windows11: true, highDPI: false, want: application.None},
		{name: "auto on Windows 10", mode: "auto", windows11: false, highDPI: true, want: application.None},
		{name: "legacy on", mode: "on", windows11: true, want: application.Mica},
		{name: "explicit mica still honored on low DPI", mode: "mica", windows11: true, highDPI: false, want: application.Mica},
		{name: "acrylic", mode: "acrylic", windows11: true, want: application.Acrylic},
		{name: "mica alt", mode: "tabbed", windows11: true, want: application.Tabbed},
		{name: "off", mode: "off", windows11: true, highDPI: true, want: application.None},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := backdropTypeForMode(tt.mode, tt.windows11, tt.highDPI); got != tt.want {
				t.Fatalf("backdropTypeForMode(%q, %v, %v) = %v, want %v", tt.mode, tt.windows11, tt.highDPI, got, tt.want)
			}
		})
	}
}
