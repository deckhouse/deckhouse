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

package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

func TestState_BashibleStepsStatus_EmptyWhenNothingSaved(t *testing.T) {
	s := NewBootstrapState(cache.NewTestCache())

	statuses, err := s.BashibleStepsStatus(t.Context())
	require.NoError(t, err)
	require.Empty(t, statuses)
}

func TestState_BashibleStepsStatus_RoundTrip(t *testing.T) {
	s := NewBootstrapState(cache.NewTestCache())

	want := map[string]string{
		"000_step_one": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"001_step_two": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}

	require.NoError(t, s.SaveBashibleStepsStatus(t.Context(), want))

	got, err := s.BashibleStepsStatus(t.Context())
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestState_RegistryPKI_NotFoundWhenNothingSaved(t *testing.T) {
	s := NewBootstrapState(cache.NewTestCache())

	pki, ok, err := s.RegistryPKI(t.Context())
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, registry.PKI{}, pki)
}

func TestState_RegistryPKI_RoundTrip(t *testing.T) {
	s := NewBootstrapState(cache.NewTestCache())

	want := registry.PKI{
		CA: registry.PKICertKey{Cert: "cert-data", Key: "key-data"},
		ROUser: registry.PKIUser{
			Name: "ro", Password: "ro-pass", PasswordHash: "ro-hash",
		},
		RWUser: registry.PKIUser{
			Name: "rw", Password: "rw-pass", PasswordHash: "rw-hash",
		},
	}

	require.NoError(t, s.SaveRegistryPKI(t.Context(), want))

	got, ok, err := s.RegistryPKI(t.Context())
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, want, got)
}
