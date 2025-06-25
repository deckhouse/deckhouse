/*
Copyright 2025 Flant JSC

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

package registry

import (
	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRegistryDataLoadFromInput(t *testing.T) {
	tests := []struct {
		name                    string
		deckhouseRegistrySecret deckhouseRegistrySecret
		bashibleConfigSecret    *bashibleConfigSecret
		wantRegistryData        *RegistryData
		wantErr                 bool
	}{
		{
			name: "Empty registry bashible config",
			deckhouseRegistrySecret: deckhouseRegistrySecret{
				Address: "registry-1.com",
				Path:    "/test",
				Scheme:  "https",
			},
			bashibleConfigSecret: nil,
			wantRegistryData: &RegistryData{
				RegistryModuleEnable: false,
				Mode:                 "unmanaged",
				ImagesBase:           "registry-1.com/test",
				Version:              "unknown",
				Hosts: map[string]bashible.ContextHosts{
					"registry-1.com": {
						Mirrors: []bashible.ContextMirrorHost{{Host: "registry-1.com", Scheme: "https"}},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "With registry bashible config",
			deckhouseRegistrySecret: deckhouseRegistrySecret{
				Address: "registry-1.com",
				Path:    "/test",
				Scheme:  "https",
			},
			bashibleConfigSecret: &bashibleConfigSecret{
				Mode:           "proxy",
				ImagesBase:     "registry-2.com/test",
				Version:        "1",
				ProxyEndpoints: []string{"endpoint-1", "endpoint-2"},
				Hosts: map[string]bashible.ConfigHosts{
					"registry-2.com": {
						Mirrors: []bashible.ConfigMirrorHost{{Host: "registry-2.com", Scheme: "https"}},
					},
				},
			},
			wantRegistryData: &RegistryData{
				RegistryModuleEnable: true,
				Mode:                 "proxy",
				ImagesBase:           "registry-2.com/test",
				Version:              "1",
				ProxyEndpoints:       []string{"endpoint-1", "endpoint-2"},
				Hosts: map[string]bashible.ContextHosts{
					"registry-2.com": {
						Mirrors: []bashible.ContextMirrorHost{{Host: "registry-2.com", Scheme: "https"}},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rData := &RegistryData{}
			err := rData.loadFromInput(tt.deckhouseRegistrySecret, tt.bashibleConfigSecret)

			if tt.wantErr {
				assert.Error(t, err, "Expected an error but got none")
			} else {
				assert.NoError(t, err, "Expected no error but got one")
			}

			assert.Equal(t, tt.wantRegistryData, rData, "Expected and actual configurations do not match")
		})
	}
}
