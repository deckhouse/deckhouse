/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1

import "testing"

func TestHasMasterNodeGroup(t *testing.T) {
	tests := []struct {
		name   string
		config *ZvirtProviderClusterConfiguration
		want   bool
	}{
		{name: "nil receiver", config: nil, want: false},
		{name: "empty config", config: &ZvirtProviderClusterConfiguration{}, want: false},
		{
			name:   "master node group set",
			config: &ZvirtProviderClusterConfiguration{MasterNodeGroup: ZvirtMasterNodeGroup{Replicas: 1}},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.HasMasterNodeGroup(); got != tt.want {
				t.Fatalf("HasMasterNodeGroup() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeGroupNames(t *testing.T) {
	tests := []struct {
		name   string
		config *ZvirtProviderClusterConfiguration
		want   []string
	}{
		{name: "nil receiver", config: nil, want: nil},
		{name: "no node groups", config: &ZvirtProviderClusterConfiguration{}, want: nil},
		{
			name: "two node groups",
			config: &ZvirtProviderClusterConfiguration{NodeGroups: []ZvirtStaticNodeGroup{
				{Name: "worker"},
				{Name: "front"},
			}},
			want: []string{"worker", "front"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.NodeGroupNames()
			if len(got) != len(tt.want) {
				t.Fatalf("NodeGroupNames() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("NodeGroupNames()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
