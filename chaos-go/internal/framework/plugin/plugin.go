package plugin

import (
	"chaos-go/internal/framework/config"
	"chaos-go/internal/framework/scheduler"
	"chaos-go/internal/framework/web"
)

type Meta struct {
	ID           string
	Name         string
	Version      string
	Dependencies []string

	SoftDeps []string

	Disabled bool
}

func (m Meta) IsEnabled() bool { return !m.Disabled }

type Registration struct {
	Meta           Meta
	Models         []any
	Configure      func(cfg *config.AppConfig) error
	RegisterRoutes func(rg *web.RouterGroup)
	Tasks          []scheduler.TaskSpec
	Start          func() error
	Stop           func() error
}
