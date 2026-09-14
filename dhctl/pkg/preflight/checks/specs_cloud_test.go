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

package checks

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

//go:embed testdata/specs_test_pcc_for_happy_path.yml
var validPCC []byte

//go:embed testdata/specs_test_invalid_pcc.yml
var invalidPCC []byte

//go:embed testdata/specs_test_malformed_pcc.yml
var malformedPCC []byte

//go:embed testdata/specs_test_quoted_number_pcc.yml
var quotedNumberPCC []byte

func assertNotApplicable(t assert.TestingT, err error, i ...any) bool {
	return assert.ErrorIs(t, err, preflight.ErrNotApplicable, i...)
}

func TestCloudSystemRequirementsCheck(t *testing.T) {
	tests := []struct {
		name           string
		installConfig  *config.DeckhouseInstaller
		assertionCheck assert.ErrorAssertionFunc
	}{
		{
			name: "happy path",
			installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: validPCC,
			},
			assertionCheck: assert.NoError,
		},
		{
			name: "invalid ProviderClusterConfiguration",
			installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: invalidPCC,
			},
			assertionCheck: func(t assert.TestingT, err error, i ...any) bool {
				// All three violations at once: reporting them one per run cost a bootstrap
				// attempt per number.
				return assert.ErrorContains(t, err, "numCPUs: 2 configured, at least 4 required") &&
					assert.ErrorContains(t, err, "memory: 2 MB configured, at least 7680 MB required") &&
					assert.ErrorContains(t, err, "rootDiskSizeGb: 16 GB configured, at least 50 GB required")
			},
		},
		{
			name: "malformed ProviderClusterConfiguration",
			installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: malformedPCC,
			},
			assertionCheck: func(t assert.TestingT, err error, i ...any) bool {
				var failure *preflight.Failure
				if !assert.ErrorAs(t, err, &failure) {
					return false
				}
				// The field path is in `checked:`, which is what the reader edits.
				return assert.Equal(t, "ZvirtClusterConfiguration.masterNodeGroup.instanceClass.memory", failure.Checked) &&
					assert.Equal(t, "the field is not set", failure.Observed)
			},
		},
		{
			// mc-flow guard: with no PCC supplied (master sizing lives in
			// NodeGroup/InstanceClass resources resolved by the external
			// validator) there is nothing here for this check to read. That is
			// reported as not applicable rather than as a pass: the check used
			// to print a ✓, and have the runner remember it, for a master it had
			// never looked at.
			name: "mc-flow: nil ProviderClusterConfig is not applicable",
			installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: nil,
			},
			assertionCheck: assertNotApplicable,
		},
		{
			name: "mc-flow: empty ProviderClusterConfig is not applicable",
			installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: []byte{},
			},
			assertionCheck: assertNotApplicable,
		},
		{
			// A quoted number is what YAML gives for `memory: "8192"`, and the check used to
			// assert propertyValue.(int) on it and panic — taking down the run it was checking,
			// with a Go stack trace in place of the sentence naming the field.
			name: "a quoted number is an error, not a panic",
			installConfig: &config.DeckhouseInstaller{
				ProviderClusterConfig: quotedNumberPCC,
			},
			assertionCheck: func(t assert.TestingT, err error, i ...any) bool {
				return assert.ErrorContains(t, err, `"8192" is a string; expected a number`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := CloudSystemRequirementsCheck{
				InstallConfig: tt.installConfig,
			}

			_, err := check.Run(t.Context())

			tt.assertionCheck(t, err, "CloudSystemRequirementsCheck.Run()")
		})
	}
}
