// Copyright 2024 Flant JSC
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
	_ "embed"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

//go:embed testdata/specs_test_pcc_for_happy_path.yml
var validPCC []byte

//go:embed testdata/specs_test_invalid_pcc.yml
var invalidPCC []byte

//go:embed testdata/specs_test_malformed_pcc.yml
var malformedPCC []byte

//go:embed testdata/specs_test_minimal_pcc.yml
var minimalPCC []byte

func TestCloudMasterNodeSystemRequirementsCheck(t *testing.T) {
	type fields struct {
		installConfig *config.DeckhouseInstaller
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "happy path",
			fields: fields{installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: validPCC,
				Bundle:                config.DefaultBundle,
			}},
			wantErr: assert.NoError,
		},
		{
			name: "minimal bundle happy path",
			fields: fields{installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: minimalPCC,
				Bundle:                config.MinimalBundle,
			}},
			wantErr: assert.NoError,
		},
		{
			name: "minimal node sizing fails for default bundle",
			fields: fields{installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: minimalPCC,
				Bundle:                config.DefaultBundle,
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "expected at least")
			},
		},
		{
			name: "minimal bundle fails with less than 2 CPU",
			fields: fields{installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: minimalZvirtPCC(1, 4096, 30),
				Bundle:                config.MinimalBundle,
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "CPU cores count")
			},
		},
		{
			name: "minimal bundle fails with less than 4GiB RAM",
			fields: fields{installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: minimalZvirtPCC(2, 3072, 30),
				Bundle:                config.MinimalBundle,
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "RAM amount")
			},
		},
		{
			name: "minimal bundle fails with less than 30GiB root disk",
			fields: fields{installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: minimalZvirtPCC(2, 4096, 20),
				Bundle:                config.MinimalBundle,
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "Root disk capacity")
			},
		},
		{
			name: "invalid ProviderClusterConfiguration",
			fields: fields{installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: invalidPCC,
				Bundle:                config.DefaultBundle,
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "expected at least")
			},
		},
		{
			name: "malformed ProviderClusterConfiguration",
			fields: fields{installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: malformedPCC,
				Bundle:                config.DefaultBundle,
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "malformed provider cluster configuration")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &Checker{
				installConfig: tt.fields.installConfig,
			}
			tt.wantErr(t,
				pc.CheckCloudMasterNodeSystemRequirements(context.Background()),
				"CheckCloudMasterNodeSystemRequirements()",
			)
		})
	}
}

func minimalZvirtPCC(cpuCores, memoryMB, rootDiskSizeGB int) []byte {
	return []byte(fmt.Sprintf(`
apiVersion: deckhouse.io/v1
kind: ZvirtClusterConfiguration
layout: Standard
clusterID: b46372e7-0d52-40c7-9bbf-fda31e187088
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: %d
    memory: %d
    rootDiskSizeGb: %d
    template: debian-bookworm
    vnicProfileID: 49bb4594-0cd4-4eb7-8288-8594eafd5a86
    storageDomainID: c4bf82a5-b803-40c3-9f6c-b9398378f424
nodeGroups:
  - name: worker
    replicas: 1
    instanceClass:
      numCPUs: 2
      memory: 4096
      template: debian-bookworm
      vnicProfileID: 49bb4594-0cd4-4eb7-8288-8594eafd5a86
provider:
  server: "<SERVER>"
  username: "<USERNAME>"
  password: "<PASSWORD>"
  insecure: true
`, cpuCores, memoryMB, rootDiskSizeGB))
}
