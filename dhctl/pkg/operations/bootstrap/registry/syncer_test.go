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

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// TestBuildCacheFillSyncerConfig verifies that the pure config-builder produces
// a well-formed SyncerConfig with the on-node seed as source and the cache leader
// as destination. No cluster, no SSH, no k8s: this is a pure unit test.
func TestBuildCacheFillSyncerConfig(t *testing.T) {
	const (
		testCA     = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"
		rwUser     = "cache-rw"
		rwPassword = "rw-pass"
	)
	cfg := BuildCacheFillSyncerConfig(testCA, rwUser, rwPassword)

	// Source = the on-node seed (local on the master), RW creds (catalog needs the
	// `*` action, RO-only would 401 on GET /v2/_catalog), module CA.
	if cfg.Src.Address != "127.0.0.1:5010" {
		t.Fatalf("Src.Address = %q, want 127.0.0.1:5010 (seed)", cfg.Src.Address)
	}
	if cfg.Src.CA != testCA {
		t.Fatalf("Src.CA mismatch (seed is https with module CA)")
	}
	if cfg.Src.User == nil || cfg.Src.User.Name != rwUser || cfg.Src.User.Password != rwPassword {
		t.Fatalf("Src.User = %+v, want seed RW creds (catalog needs *)", cfg.Src.User)
	}
	// Dest = the cache leader, RW creds, module CA.
	if cfg.Dest.Address != "registry-cache-leader.d8-system.svc:5001" {
		t.Fatalf("Dest.Address = %q, want cache leader", cfg.Dest.Address)
	}
	if cfg.Dest.CA != testCA {
		t.Fatalf("Dest.CA mismatch (cache dest is https with module CA)")
	}
	if cfg.Dest.User == nil || cfg.Dest.User.Name != rwUser || cfg.Dest.User.Password != rwPassword {
		t.Fatalf("Dest.User = %+v, want cache RW creds", cfg.Dest.User)
	}
	if cfg.Prune {
		t.Fatalf("Prune = true, want false (additive bootstrap fill)")
	}

	t.Run("config round-trips through YAML", func(t *testing.T) {
		data, err := yaml.Marshal(cfg)
		require.NoError(t, err, "should marshal to YAML without error")

		var decoded SyncerConfig
		require.NoError(t, yaml.Unmarshal(data, &decoded),
			"should unmarshal from YAML without error")

		require.Equal(t, cfg, decoded, "YAML round-trip should be lossless")
	})

	t.Run("YAML source key is 'source' (syncer expects this field name)", func(t *testing.T) {
		data, err := yaml.Marshal(cfg)
		require.NoError(t, err)
		require.Contains(t, string(data), "source:", "YAML must use 'source' key")
	})

	t.Run("YAML destination key is 'destination' (syncer expects this field name)", func(t *testing.T) {
		data, err := yaml.Marshal(cfg)
		require.NoError(t, err)
		require.Contains(t, string(data), "destination:", "YAML must use 'destination' key")
	})
}
