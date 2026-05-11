package fnqpowermode

import "github.com/TIANLI0/BS2PRO-Controller/internal/types"

const (
	PluginID   = "lenovo-legion-fnq-power-mode"
	PluginName = "Lenovo Legion Fn+Q Power Mode"
)

// PowerModeState is emitted when the Lenovo firmware reports a power-mode change.
type PowerModeState struct {
	Raw       int    `json:"raw"`
	Mapped    int    `json:"mapped"`
	Mode      string `json:"mode"`
	Source    string `json:"source"`
	Timestamp int64  `json:"timestamp"`
}

type Options struct {
	Logger       types.Logger
	OnModeChange func(PowerModeState)
}

func modeName(mapped int) string {
	switch mapped {
	case 0:
		return "Quiet"
	case 1:
		return "Balance"
	case 2:
		return "Performance"
	case 223:
		return "Extreme"
	case 254:
		return "GodMode"
	default:
		return "Unknown"
	}
}
