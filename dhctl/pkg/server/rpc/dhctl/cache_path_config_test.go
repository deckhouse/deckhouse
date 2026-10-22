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

package dhctl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCachePathConfig(t *testing.T) {
	const (
		clusterConfig = "kind: ClusterConfiguration\ncloud:\n  provider: DVP"
		providerMC    = "apiVersion: deckhouse.io/v1alpha1\nkind: ModuleConfig\nmetadata:\n  name: cloud-provider-dvp"
		globalMC      = "apiVersion: deckhouse.io/v1alpha1\nkind: ModuleConfig\nmetadata:\n  name: global\nspec:\n  settings:\n    prefix: dev"
		secret        = "apiVersion: v1\nkind: Secret\nmetadata:\n  name: d8-credentials"
	)

	tests := []struct {
		name      string
		resources string
		configs   []string
		want      string
	}{
		{
			name:      "the global ModuleConfig is taken from the resources, the other documents are not",
			resources: providerMC + "\n---\n" + secret + "\n---\n" + globalMC,
			configs:   []string{clusterConfig, ""},
			want:      clusterConfig + "\n---\n" + globalMC + "\n",
		},
		{
			name:      "no resources, as sent by an old Commander",
			resources: "",
			configs:   []string{clusterConfig, "kind: DVPClusterConfiguration"},
			want:      clusterConfig + "\n---\nkind: DVPClusterConfiguration\n",
		},
		{
			name:      "resources without the global ModuleConfig",
			resources: providerMC + "\n---\n" + secret,
			configs:   []string{clusterConfig},
			want:      clusterConfig + "\n",
		},
		{
			name:      "the configs keep their order, the global ModuleConfig goes last",
			resources: globalMC,
			configs:   []string{"a: 1", "b: 2", "c: 3"},
			want:      "a: 1\n---\nb: 2\n---\nc: 3\n---\n" + globalMC + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, cachePathConfig(tt.resources, tt.configs...))
		})
	}
}
