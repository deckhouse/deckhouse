// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package options

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type VaultOption func(*VaultOptions)

type VaultOptions struct {
	Logger   *log.Logger
	Registry *prometheus.Registry
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
		v.Logger = logger
	}
}

// WithRegistry sets an existing registry for the GroupedVault.
func WithRegistry(registry *prometheus.Registry) VaultOption {
	return func(v *VaultOptions) {
		v.Registry = registry
	}
}

// WithNewRegistry creates a new registry for the GroupedVault.
func WithNewRegistry() VaultOption {
	return func(v *VaultOptions) {
		v.Registry = prometheus.NewRegistry()
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
