package vault

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type VaultOption func(*VaultOptions)

type VaultOptions struct {
	logger   *log.Logger
	registry *prometheus.Registry
}

func NewVaultOptions(opts ...VaultOption) *VaultOptions {
	v := &VaultOptions{}

	for _, option := range opts {
		option(v)
	}

	return v
}

// WithLogger sets the logger for the GroupedVault.
func WithLogger(logger *log.Logger) VaultOption {
	return func(v *VaultOptions) {
		v.logger = logger
	}
}

// WithRegistry sets an existing registry for the GroupedVault.
func WithRegistry(registry *prometheus.Registry) VaultOption {
	return func(v *VaultOptions) {
		v.registry = registry
	}
}

// WithNewRegistry creates a new registry for the GroupedVault.
func WithNewRegistry() VaultOption {
	return func(v *VaultOptions) {
		v.registry = prometheus.NewRegistry()
	}
}

type RegisterOption func(*RegisterOptions)

type RegisterOptions struct {
	Help           string
	ConstantLabels map[string]string
}

func NewRegisterOptions(opts ...RegisterOption) *RegisterOptions {
	v := &RegisterOptions{}

	for _, option := range opts {
		option(v)
	}

	return v
}

func WithHelp(help string) RegisterOption {
	return func(v *RegisterOptions) {
		v.Help = help
	}
}

func WithConstantLabels(labels map[string]string) RegisterOption {
	return func(v *RegisterOptions) {
		v.ConstantLabels = labels
	}
}
