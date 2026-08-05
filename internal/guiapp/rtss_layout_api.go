package guiapp

import "github.com/TIANLI0/THRM/internal/rtss"

// GetRTSSLayoutStatus inspects the active OverlayEditor layout without writing
// any RTSS files. It is intentionally a GUI-side helper so setup never depends
// on the core service being connected to a cooler.
func (a *App) GetRTSSLayoutStatus() rtss.LayoutStatus {
	return rtss.InspectLayout()
}

// CreateRTSSAnchor adds one idempotent 1% empty layer to the active layout.
// The RTSS layout is backed up before any write; ambiguous existing candidates
// are reported to the UI instead of being duplicated or renamed silently.
func (a *App) CreateRTSSAnchor() (rtss.LayoutStatus, error) {
	return rtss.CreateAnchor()
}
