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

package config

import (
	"encoding/json"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func metaConfigWithNetwork(t *testing.T, cc map[string]string, mcNetwork map[string]interface{}) *MetaConfig {
	t.Helper()

	clusterConfig := map[string]json.RawMessage{}
	for k, v := range cc {
		encoded, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal %s: %v", k, err)
		}
		clusterConfig[k] = encoded
	}

	m := &MetaConfig{ClusterConfig: clusterConfig}
	if mcNetwork != nil {
		m.ModuleConfigs = []*ModuleConfig{{
			ObjectMeta: metav1.ObjectMeta{Name: "control-plane-manager"},
			Spec:       ModuleConfigSpec{Settings: SettingsValues{"network": mcNetwork}},
		}}
	}
	return m
}

func TestNetwork_Precedence(t *testing.T) {
	tests := []struct {
		name string
		cc   map[string]string
		mc   map[string]interface{}
		want NetworkSettings
	}{
		{
			name: "ClusterConfiguration only",
			cc:   map[string]string{"podSubnetCIDR": "10.111.0.0/16", "serviceSubnetCIDR": "10.222.0.0/16", "podSubnetNodeCIDRPrefix": "23"},
			want: NetworkSettings{PodSubnetCIDR: "10.111.0.0/16", ServiceSubnetCIDR: "10.222.0.0/16", PodSubnetNodeCIDRPrefix: "23"},
		},
		{
			name: "ModuleConfig only",
			mc:   map[string]interface{}{"podSubnetCIDR": "10.111.0.0/16", "serviceSubnetCIDR": "10.222.0.0/16", "podSubnetNodeCIDRPrefix": "23"},
			want: NetworkSettings{PodSubnetCIDR: "10.111.0.0/16", ServiceSubnetCIDR: "10.222.0.0/16", PodSubnetNodeCIDRPrefix: "23"},
		},
		{
			name: "ModuleConfig wins over ClusterConfiguration",
			cc:   map[string]string{"podSubnetCIDR": "10.99.0.0/16", "serviceSubnetCIDR": "10.88.0.0/16", "podSubnetNodeCIDRPrefix": "22"},
			mc:   map[string]interface{}{"podSubnetCIDR": "10.111.0.0/16", "serviceSubnetCIDR": "10.222.0.0/16", "podSubnetNodeCIDRPrefix": "23"},
			want: NetworkSettings{PodSubnetCIDR: "10.111.0.0/16", ServiceSubnetCIDR: "10.222.0.0/16", PodSubnetNodeCIDRPrefix: "23"},
		},
		{
			name: "a half-migrated cluster resolves each parameter independently",
			cc:   map[string]string{"podSubnetCIDR": "10.99.0.0/16", "serviceSubnetCIDR": "10.88.0.0/16"},
			mc:   map[string]interface{}{"podSubnetNodeCIDRPrefix": "23"},
			want: NetworkSettings{PodSubnetCIDR: "10.99.0.0/16", ServiceSubnetCIDR: "10.88.0.0/16", PodSubnetNodeCIDRPrefix: "23"},
		},
		{
			name: "prefix defaults to 24 when set nowhere",
			cc:   map[string]string{"podSubnetCIDR": "10.99.0.0/16", "serviceSubnetCIDR": "10.88.0.0/16"},
			want: NetworkSettings{PodSubnetCIDR: "10.99.0.0/16", ServiceSubnetCIDR: "10.88.0.0/16", PodSubnetNodeCIDRPrefix: DefaultPodSubnetNodeCIDRPrefix},
		},
		{
			name: "neither document set: both CIDRs stay empty, no invented default",
			want: NetworkSettings{PodSubnetNodeCIDRPrefix: DefaultPodSubnetNodeCIDRPrefix},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := metaConfigWithNetwork(t, tt.cc, tt.mc)
			got := m.Network()
			if got != tt.want {
				t.Fatalf("Network() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRequireNetwork(t *testing.T) {
	t.Run("both CIDRs set", func(t *testing.T) {
		m := metaConfigWithNetwork(t, map[string]string{"podSubnetCIDR": "10.111.0.0/16", "serviceSubnetCIDR": "10.222.0.0/16"}, nil)
		if err := m.RequireNetwork(); err != nil {
			t.Fatalf("RequireNetwork: %v", err)
		}
	})

	t.Run("resolved from ModuleConfig alone", func(t *testing.T) {
		m := metaConfigWithNetwork(t, nil, map[string]interface{}{"podSubnetCIDR": "10.111.0.0/16", "serviceSubnetCIDR": "10.222.0.0/16"})
		if err := m.RequireNetwork(); err != nil {
			t.Fatalf("RequireNetwork: %v", err)
		}
	})

	t.Run("one CIDR missing everywhere", func(t *testing.T) {
		m := metaConfigWithNetwork(t, map[string]string{"podSubnetCIDR": "10.111.0.0/16"}, nil)
		err := m.RequireNetwork()
		if err == nil {
			t.Fatal("expected an error naming serviceSubnetCIDR")
		}
		if got, want := err.Error(), "serviceSubnetCIDR"; !strings.Contains(got, want) {
			t.Fatalf("error = %q, want it to mention %q", got, want)
		}
	})

	t.Run("both CIDRs missing everywhere", func(t *testing.T) {
		m := metaConfigWithNetwork(t, nil, nil)
		err := m.RequireNetwork()
		if err == nil {
			t.Fatal("expected an error naming both CIDRs")
		}
		for _, want := range []string{"podSubnetCIDR", "serviceSubnetCIDR"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error = %q, want it to mention %q", err.Error(), want)
			}
		}
	})
}

