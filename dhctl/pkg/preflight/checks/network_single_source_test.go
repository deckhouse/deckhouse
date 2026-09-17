// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package checks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestNetworkSingleSourceCheck(t *testing.T) {
	tests := []struct {
		name       string
		metaConfig *config.MetaConfig
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name: "set only in ClusterConfiguration",
			metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"podSubnetCIDR":     []byte(`"10.111.0.0/16"`),
					"serviceSubnetCIDR": []byte(`"10.222.0.0/16"`),
				},
			},
			wantErr: assert.NoError,
		},
		{
			name:       "set only in ModuleConfig",
			metaConfig: metaConfigWithModuleConfigNetwork(t, map[string]interface{}{"podSubnetCIDR": "10.111.0.0/16"}),
			wantErr:    assert.NoError,
		},
		{
			name:       "set nowhere",
			metaConfig: &config.MetaConfig{},
			wantErr:    assert.NoError,
		},
		{
			name: "podSubnetCIDR set in both documents",
			metaConfig: func() *config.MetaConfig {
				m := metaConfigWithModuleConfigNetwork(t, map[string]interface{}{"podSubnetCIDR": "10.111.0.0/16"})
				m.ClusterConfig = map[string]json.RawMessage{"podSubnetCIDR": []byte(`"10.99.0.0/16"`)}
				return m
			}(),
			wantErr: func(t assert.TestingT, err error, i ...any) bool {
				return assert.ErrorContains(t, err, "podSubnetCIDR")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := NetworkSingleSourceCheck{MetaConfig: tt.metaConfig}
			tt.wantErr(t, check.Run(t.Context()))
		})
	}
}
