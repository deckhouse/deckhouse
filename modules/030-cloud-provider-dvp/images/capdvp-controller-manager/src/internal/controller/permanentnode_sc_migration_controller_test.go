/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePermanentNodeProviderConfig(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		cfg, err := parsePermanentNodeProviderConfig(nil)
		require.NoError(t, err)
		assert.Nil(t, cfg.MasterNodeGroup)
		assert.Empty(t, cfg.NodeGroups)
	})

	t.Run("master with all disk types", func(t *testing.T) {
		raw := []byte(`
masterNodeGroup:
  instanceClass:
    rootDisk:
      storageClass: local
    etcdDisk:
      storageClass: replicated
    additionalDisks:
      - storageClass: fast
      - storageClass: slow
`)
		cfg, err := parsePermanentNodeProviderConfig(raw)
		require.NoError(t, err)
		require.NotNil(t, cfg.MasterNodeGroup)

		ic := cfg.MasterNodeGroup.InstanceClass
		assert.Equal(t, "local", scString(ic.RootDisk))
		assert.Equal(t, "replicated", scString(ic.EtcdDisk))
		assert.Equal(t, []string{"fast", "slow"}, scSlice(ic.AdditionalDisks))
	})

	t.Run("master without optional SC fields", func(t *testing.T) {
		raw := []byte(`
masterNodeGroup:
  instanceClass: {}
`)
		cfg, err := parsePermanentNodeProviderConfig(raw)
		require.NoError(t, err)
		require.NotNil(t, cfg.MasterNodeGroup)
		assert.Empty(t, scString(cfg.MasterNodeGroup.InstanceClass.RootDisk))
		assert.Empty(t, scString(cfg.MasterNodeGroup.InstanceClass.EtcdDisk))
	})

	t.Run("static node groups", func(t *testing.T) {
		raw := []byte(`
nodeGroups:
  - name: infra
    instanceClass:
      rootDisk:
        storageClass: local
      additionalDisks:
        - storageClass: replicated
  - name: gpu
    instanceClass:
      rootDisk:
        storageClass: gpu-fast
`)
		cfg, err := parsePermanentNodeProviderConfig(raw)
		require.NoError(t, err)
		require.Len(t, cfg.NodeGroups, 2)

		assert.Equal(t, "infra", cfg.NodeGroups[0].Name)
		assert.Equal(t, "local", scString(cfg.NodeGroups[0].InstanceClass.RootDisk))
		assert.Equal(t, []string{"replicated"}, scSlice(cfg.NodeGroups[0].InstanceClass.AdditionalDisks))

		assert.Equal(t, "gpu", cfg.NodeGroups[1].Name)
		assert.Equal(t, "gpu-fast", scString(cfg.NodeGroups[1].InstanceClass.RootDisk))
	})

	t.Run("invalid yaml returns error", func(t *testing.T) {
		_, err := parsePermanentNodeProviderConfig([]byte("}{invalid"))
		assert.Error(t, err)
	})
}

func TestDesiredSCForDisk(t *testing.T) {
	cfg := nodeGroupSCConfig{
		RootDiskSC:       "local",
		EtcdDiskSC:       "replicated",
		AdditionalDiskSC: []string{"fast", "slow"},
	}

	tests := []struct {
		name     string
		diskName string
		want     string
	}{
		{
			name:     "root disk",
			diskName: "prefix-master-0-a1b2c3",
			want:     "local",
		},
		{
			name:     "kubernetes-data disk",
			diskName: "prefix-master-kubernetes-data-0-d4e5f6",
			want:     "replicated",
		},
		{
			name:     "additional disk index 0",
			diskName: "prefix-master-additional-disk-0-0-g7h8i9",
			want:     "fast",
		},
		{
			name:     "additional disk index 1",
			diskName: "prefix-master-additional-disk-1-0-j0k1l2",
			want:     "slow",
		},
		{
			name:     "additional disk index out of range",
			diskName: "prefix-master-additional-disk-5-0-m3n4o5",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := desiredSCForDisk(tt.diskName, cfg)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDesiredSCForDisk_NullSC(t *testing.T) {
	cfg := nodeGroupSCConfig{
		RootDiskSC: "",
		EtcdDiskSC: "",
	}

	assert.Empty(t, desiredSCForDisk("prefix-master-0-abc123", cfg), "empty SC should be skipped")
	assert.Empty(t, desiredSCForDisk("prefix-master-kubernetes-data-0-abc123", cfg))
}

func TestParseAdditionalDiskIndex(t *testing.T) {
	tests := []struct {
		name     string
		diskName string
		wantIdx  int
		wantOK   bool
	}{
		{"index 0", "prefix-master-additional-disk-0-0-hash", 0, true},
		{"index 3", "prefix-ng-additional-disk-3-1-hash", 3, true},
		{"root disk", "prefix-master-0-hash", 0, false},
		{"kubernetes-data disk", "prefix-master-kubernetes-data-0-hash", 0, false},
		{"empty", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, ok := parseAdditionalDiskIndex(tt.diskName)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantIdx, idx)
			}
		})
	}
}
