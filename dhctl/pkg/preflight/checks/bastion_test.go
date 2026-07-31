// Copyright 2026 Flant JSC
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

package checks

import (
	"testing"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

func TestBastionAvailabilityDescription(t *testing.T) {
	const withBastionDesc = "ssh connection to the bastion host is possible"
	const noBastionDesc = "no bastion configured, skipping bastion availability check"

	tests := []struct {
		name string
		cfg  *sshconfig.ConnectionConfig
		want string
	}{
		{"bastion set", &sshconfig.ConnectionConfig{Config: &sshconfig.Config{BastionHost: "10.0.0.1"}}, withBastionDesc},
		{"no bastion", &sshconfig.ConnectionConfig{Config: &sshconfig.Config{}}, noBastionDesc},
		{"nil inner config", &sshconfig.ConnectionConfig{}, noBastionDesc},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init := providerinitializer.NewSSHProviderInitializer(nil, tt.cfg)
			check := BastionAvailabilityCheck{SSHProviderInitializer: init}
			if got := check.Description(); got != tt.want {
				t.Errorf("Description() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBastionConfigured(t *testing.T) {
	tests := []struct {
		name string
		cfg  *sshconfig.Config
		want bool
	}{
		{"nil config", nil, false},
		{"empty bastion host", &sshconfig.Config{}, false},
		{"bastion host set", &sshconfig.Config{BastionHost: "10.0.0.1"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bastionConfigured(tt.cfg); got != tt.want {
				t.Errorf("bastionConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBastionPort(t *testing.T) {
	port := 2222
	tests := []struct {
		name string
		cfg  *sshconfig.Config
		want string
	}{
		{"nil config", nil, "22"},
		{"nil port defaults to 22", &sshconfig.Config{BastionHost: "h"}, "22"},
		{"port set", &sshconfig.Config{BastionHost: "h", BastionPort: &port}, "2222"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bastionPort(tt.cfg); got != tt.want {
				t.Errorf("bastionPort() = %v, want %v", got, tt.want)
			}
		})
	}
}
