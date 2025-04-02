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

//go:embed testdata/specs_test_yandex_pcc_for_happy_path.yml
var validYandexPCC []byte

//go:embed testdata/specs_test_yandex_pcc_empty_withNATInstance.yml
var yandexPccEmptyNatInstance []byte

func TestCheckYandexWithNatInstanceConfig(t *testing.T) {
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
				ProviderClusterConfig: validYandexPCC,
			}},
			wantErr: assert.NoError,
		},
		{
			name: "other cloud provider",
			fields: fields{installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: validPCC,
			}},
			wantErr: assert.NoError,
		},
		{
			name: "empty withNATInstance",
			fields: fields{installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: yandexPccEmptyNatInstance,
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "neither internalSubnetCIDR nor internalSubnetID are provided")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &Checker{
				installConfig: tt.fields.installConfig,
			}
			tt.wantErr(t,
				pc.CheckYandexWithNatInstanceConfig(context.Background()),
				fmt.Sprintf("TestCheckYandexWithNatInstanceConfig()"),
			)
		})
	}
}
