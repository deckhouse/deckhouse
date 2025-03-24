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
			}},
			wantErr: assert.NoError,
		},
		{
			name: "invalid ProviderClusterConfiguration",
			fields: fields{installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: invalidPCC,
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "expected at least")
			},
		},
		{
			name: "malformed ProviderClusterConfiguration",
			fields: fields{installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: malformedPCC,
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
				fmt.Sprintf("CheckCloudMasterNodeSystemRequirements()"),
			)
		})
	}
}
