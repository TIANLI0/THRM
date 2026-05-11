//go:build !windows

package fnqpowermode

import (
	"context"

	"github.com/TIANLI0/BS2PRO-Controller/internal/plugins"
)

type Plugin struct{}

func New(options Options) *Plugin {
	return &Plugin{}
}

func (p *Plugin) ID() string {
	return PluginID
}

func (p *Plugin) Name() string {
	return PluginName
}

func (p *Plugin) Start(ctx context.Context) error {
	return nil
}

func (p *Plugin) Stop() error {
	return nil
}

func (p *Plugin) Status() plugins.Status {
	return plugins.Status{ID: p.ID(), Name: p.Name(), Running: false, LastError: "unsupported platform"}
}
