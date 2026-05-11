package plugins

import "context"

// Plugin is the lifecycle contract for optional background integrations.
type Plugin interface {
	ID() string
	Name() string
	Start(ctx context.Context) error
	Stop() error
	Status() Status
}

// Status describes the runtime state of a plugin.
type Status struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Running   bool   `json:"running"`
	LastError string `json:"lastError,omitempty"`
}
