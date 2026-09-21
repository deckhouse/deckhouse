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

package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestSplitHostPath(t *testing.T) {
	tests := []struct {
		in       string
		wantHost string
		wantPath string
	}{
		{"registry.example.com", "registry.example.com", ""},
		{"registry.example.com/", "registry.example.com", ""},
		{"registry.example.com/deckhouse", "registry.example.com", "deckhouse"},
		{"registry.example.com/deckhouse/fe", "registry.example.com", "deckhouse/fe"},
		{"registry.example.com/deckhouse/fe/modules", "registry.example.com", "deckhouse/fe/modules"},
		// Leading and trailing slashes are trimmed before splitting.
		{"/registry.example.com/x/", "registry.example.com", "x"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			host, path := splitHostPath(tt.in)
			assert.Equal(t, tt.wantHost, host)
			assert.Equal(t, tt.wantPath, path)
		})
	}
}

// TestNewRegistryClientPath checks that the repo is turned into a client
// addressing that exact path. Auth wiring is opaque from the outside, so the
// path is what is asserted here.
func TestNewRegistryClientPath(t *testing.T) {
	tests := []struct {
		repo string
		want string
	}{
		{"registry.example.com", "registry.example.com"},
		{"registry.example.com/deckhouse/fe", "registry.example.com/deckhouse/fe"},
		{"registry.example.com/deckhouse/fe/modules", "registry.example.com/deckhouse/fe/modules"},
	}

	for _, tt := range tests {
		t.Run(tt.repo, func(t *testing.T) {
			cli, err := newRegistryClient(tt.repo, &utils.RegistryConfig{}, log.NewNop())
			require.NoError(t, err)
			assert.Equal(t, tt.want, cli.GetRegistry())
		})
	}
}

// TestNewRegistryClientBadDockerConfig checks that a malformed dockercfg is a
// build-time error rather than a client that fails later on every request.
func TestNewRegistryClientBadDockerConfig(t *testing.T) {
	_, err := newRegistryClient(
		"registry.example.com/deckhouse/fe",
		&utils.RegistryConfig{DockerConfig: "%%% not base64 %%%"},
		log.NewNop(),
	)
	require.Error(t, err)
}

// TestNewRegistryClientDockerConfigOK checks the happy path: a valid dockercfg
// yields a client addressing the repo.
func TestNewRegistryClientDockerConfigOK(t *testing.T) {
	// {"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}
	const dockercfg = "eyJhdXRocyI6eyJyZWdpc3RyeS5leGFtcGxlLmNvbSI6eyJhdXRoIjoiZFhObGNqcHdZWE56In19fQ=="

	cli, err := newRegistryClient(
		"registry.example.com/deckhouse/fe",
		&utils.RegistryConfig{DockerConfig: dockercfg},
		log.NewNop(),
	)
	require.NoError(t, err)
	assert.Equal(t, "registry.example.com/deckhouse/fe", cli.GetRegistry())
}
