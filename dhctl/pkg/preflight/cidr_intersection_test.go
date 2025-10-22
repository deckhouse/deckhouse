// Copyright 2025 Flant JSC
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

package preflight

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestGetCidrFromMetaConfig(t *testing.T) {
	tests := []struct {
		metaConfig          *config.MetaConfig
		expectedPodCIDR     string
		expectedServiceCIDR string
		expectError         bool
		errorMsg            string
	}{
		{
			metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"podSubnetCIDR":     json.RawMessage(`"10.0.0.0/8"`),
					"serviceSubnetCIDR": json.RawMessage(`"192.168.0.0/16"`),
				},
			},
			expectedPodCIDR:     "10.0.0.0/8",
			expectedServiceCIDR: "192.168.0.0/16",
			expectError:         false,
		},
		{
			metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"serviceSubnetCIDR": json.RawMessage(`"192.168.0.0/16"`),
				},
			},
			expectError: true,
			errorMsg:    "missing podSubnetCIDR field in ClusterConfiguration",
		},
		{
			metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"podSubnetCIDR": json.RawMessage(`"192.168.0.0/16"`),
				},
			},
			expectError: true,
			errorMsg:    "missing serviceSubnetCIDR field in ClusterConfiguration",
		},
		{
			metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{},
			},
			expectError: true,
			errorMsg:    "missing podSubnetCIDR field in ClusterConfiguration",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("PodCIDR:%s_ServiceCIDR:%s", tt.expectedPodCIDR, tt.expectedServiceCIDR), func(t *testing.T) {
			podCIDR, serviceCIDR, err := getCidrFromMetaConfig(tt.metaConfig)

			if tt.expectError {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPodCIDR, podCIDR)
				assert.Equal(t, tt.expectedServiceCIDR, serviceCIDR)
			}
		})
	}
}

func TestCidrIntersects(t *testing.T) {
	tests := []struct {
		cidr1       string
		cidr2       string
		expectError bool
		errorMsg    string
	}{
		{
			cidr1:       "10.0.0.0/8",
			cidr2:       "10.0.1.0/24",
			expectError: true,
			errorMsg:    "CIDRs 10.0.0.0/8 and 10.0.1.0/24 are intersects",
		},
		{
			cidr1:       "10.0.0.0/8",
			cidr2:       "192.168.0.0/16",
			expectError: false,
		},
		{
			cidr1:       "invalidCIDR",
			cidr2:       "10.0.0.0/8",
			expectError: true,
			errorMsg:    "invalid CIDR address: invalidCIDR",
		},
		{
			cidr1:       "10.0.0.0/8",
			cidr2:       "invalidCIDR",
			expectError: true,
			errorMsg:    "invalid CIDR address: invalidCIDR",
		},
		{
			cidr1:       "invalidCIDR1",
			cidr2:       "invalidCIDR2",
			expectError: true,
			errorMsg:    "invalid CIDR address: invalidCIDR1",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.cidr1, tt.cidr2), func(t *testing.T) {
			err := cidrIntersects(tt.cidr1, tt.cidr2)

			if tt.expectError {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckCidrIntersection(t *testing.T) {
	type fields struct {
		metaConfig *config.MetaConfig
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "happy path",
			fields: fields{metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"podSubnetCIDR":     []byte(`"10.111.0.0/16"`),
					"serviceSubnetCIDR": []byte(`"10.222.0.0/16"`),
				},
			}},
			wantErr: assert.NoError,
		},
		{
			name: "intersects subnets",
			fields: fields{metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"podSubnetCIDR":     []byte(`"10.111.0.0/17"`),
					"serviceSubnetCIDR": []byte(`"10.111.110.0/18"`),
				},
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "are intersects")
			},
		},
		{
			name: "missing podSubnetCIDR",
			fields: fields{metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"serviceSubnetCIDR": []byte(`"10.0.0.0/8"`),
				},
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "missing podSubnetCIDR field in ClusterConfiguration")
			},
		},
		{
			name: "missing serviceSubnetCIDR",
			fields: fields{metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"podSubnetCIDR": []byte(`"10.0.0.0/8"`),
				},
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "missing serviceSubnetCIDR field in ClusterConfiguration")
			},
		},
		{
			name: "invalid podSubnetCIDR",
			fields: fields{metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"podSubnetCIDR":     []byte(`"invalidCIDR"`),
					"serviceSubnetCIDR": []byte(`"10.222.0.0/16"`),
				},
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "invalid CIDR address: invalidCIDR")
			},
		},
		{
			name: "invalid serviceSubnetCIDR",
			fields: fields{metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"podSubnetCIDR":     []byte(`"10.111.0.0/16"`),
					"serviceSubnetCIDR": []byte(`"invalidCIDR"`),
				},
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "invalid CIDR address: invalidCIDR")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &Checker{
				metaConfig: tt.fields.metaConfig,
			}
			tt.wantErr(t, pc.CheckCidrIntersection(context.Background()), fmt.Sprintf("CheckCidrIntersection()"))
		})
	}
}

func TestCheckCidrIntersectionStatic(t *testing.T) {
	type fields struct {
		metaConfig *config.MetaConfig
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "happy path",
			fields: fields{metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"podSubnetCIDR":     []byte(`"10.111.0.0/16"`),
					"serviceSubnetCIDR": []byte(`"10.222.0.0/16"`),
				},
				StaticClusterConfig: map[string]json.RawMessage{
					"internalNetworkCIDRs": []byte(`["10.128.0.0/24"]`),
				},
			}},
			wantErr: assert.NoError,
		},
		{
			name: "happy path",
			fields: fields{metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"podSubnetCIDR":     []byte(`"10.111.0.0/16"`),
					"serviceSubnetCIDR": []byte(`"10.222.0.0/16"`),
				},
			}},
			wantErr: assert.NoError,
		},
		{
			name: "intersects subnets with internal",
			fields: fields{metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"podSubnetCIDR":     []byte(`"10.111.0.0/16"`),
					"serviceSubnetCIDR": []byte(`"10.222.0.0/16"`),
				},
				StaticClusterConfig: map[string]json.RawMessage{
					"internalNetworkCIDRs": []byte(`["10.0.0.0/8"]`),
				},
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "are intersects")
			},
		},
		{
			name: "intersects serviceSubnetCIDR with internalNetworkCIDRs",
			fields: fields{metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"podSubnetCIDR":     []byte(`"10.111.0.0/16"`),
					"serviceSubnetCIDR": []byte(`"10.222.0.0/16"`),
				},
				StaticClusterConfig: map[string]json.RawMessage{
					"internalNetworkCIDRs": []byte(`["10.222.128.0/24"]`),
				},
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "are intersects")
			},
		},
		{
			name: "missing podSubnetCIDR",
			fields: fields{metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"serviceSubnetCIDR": []byte(`"10.0.0.0/8"`),
				},
				StaticClusterConfig: map[string]json.RawMessage{
					"internalNetworkCIDRs": []byte(`["10.128.0.0/24"]`),
				},
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "missing podSubnetCIDR field in ClusterConfiguration")
			},
		},
		{
			name: "missing serviceSubnetCIDR",
			fields: fields{metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"podSubnetCIDR": []byte(`"10.0.0.0/8"`),
				},
				StaticClusterConfig: map[string]json.RawMessage{
					"internalNetworkCIDRs": []byte(`["10.128.0.0/24"]`),
				},
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "missing serviceSubnetCIDR field in ClusterConfiguration")
			},
		},
		{
			name: "invalid podSubnetCIDR",
			fields: fields{metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"podSubnetCIDR":     []byte(`"invalidCIDR"`),
					"serviceSubnetCIDR": []byte(`"10.222.0.0/16"`),
				},
				StaticClusterConfig: map[string]json.RawMessage{
					"internalNetworkCIDRs": []byte(`["10.128.0.0/24"]`),
				},
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "invalid CIDR address: invalidCIDR")
			},
		},
		{
			name: "invalid serviceSubnetCIDR",
			fields: fields{metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"podSubnetCIDR":     []byte(`"10.111.0.0/16"`),
					"serviceSubnetCIDR": []byte(`"invalidCIDR"`),
				},
				StaticClusterConfig: map[string]json.RawMessage{
					"internalNetworkCIDRs": []byte(`["10.128.0.0/24"]`),
				},
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "invalid CIDR address: invalidCIDR")
			},
		},
		{
			name: "invalid internalNetworkCIDRs",
			fields: fields{metaConfig: &config.MetaConfig{
				ClusterConfig: map[string]json.RawMessage{
					"podSubnetCIDR":     []byte(`"10.111.0.0/16"`),
					"serviceSubnetCIDR": []byte(`"10.222.0.0/16"`),
				},
				StaticClusterConfig: map[string]json.RawMessage{
					"internalNetworkCIDRs": []byte(`["invalidCIDR"]`),
				},
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "invalid CIDR address: invalidCIDR")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &Checker{
				metaConfig: tt.fields.metaConfig,
			}
			tt.wantErr(t,
				pc.CheckCidrIntersectionStatic(context.Background()),
				fmt.Sprintf("CheckCidrIntersectionStatic()"),
			)
		})
	}
}
