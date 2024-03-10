/*
Copyright 2022 Flant JSC

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

package deckhouse_config

import (
	"fmt"
	"testing"

	"github.com/flant/addon-operator/pkg/values/validation"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/conversion"
	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/module-manager/test/mock"
)

const (
	expectValid   = true
	expectInvalid = false
)

func TestValidatorValidateCR(t *testing.T) {
	// Latest version is 2.
	conversion.RegisterFunc("global", 1, 2, func(settings *conversion.Settings) error {
		return nil
	})

	tests := []struct {
		name     string
		manifest string
		expect   bool
	}{
		{"settings-and-version-1",
			`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    paramStr: val1
`,
			expectValid,
		},
		{
			"settings-and-version-2",
			`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 2
  settings:
    paramStr: val1
`,
			expectValid,
		},
		{
			"settings-versions-enabled",
			`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    paramStr: val1
  enabled: false
`,
			expectValid,
		},
		{
			"enabled-only",
			`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  enabled: true
`,
			expectValid,
		},
		{
			"empty spec",
			`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec: {}
`,
			expectValid,
		},

		// Invalid cases
		{
			"settings-and-version-0",
			`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 0
  settings:
    paramStr: val1
`,
			expectInvalid,
		},
		{
			"settings-without-version",
			`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings:
    paramStr: val1
`,
			expectInvalid,
		},
		{
			"settings-without-version-with-enabled",
			`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings:
    paramStr: val1
  enabled: false
`,
			expectInvalid,
		},
		{
			"empty spec.settings without version",
			`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings: {}
`,
			expectInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			v := NewConfigValidator(nil)
			cfg, err := modCfgFromYAML(tt.manifest)
			g.Expect(err).ShouldNot(HaveOccurred(), "should parse manifest: %s", tt.manifest)
			res := v.validateCR(cfg)

			switch tt.expect {
			case expectValid:
				g.Expect(res.HasError()).Should(BeFalse(), "should be valid, got error: %s", res.Error)
			case expectInvalid:
				g.Expect(res.HasError()).Should(BeTrue(), "should be invalid, got no error")
			}
		})
	}
}

func TestValidatorValidate(t *testing.T) {
	// Latest version is 2.
	conversion.RegisterFunc("global", 1, 2, func(settings *conversion.Settings) error {
		return nil
	})

	tests := []struct {
		name     string
		manifest string
		expect   bool
	}{
		{"settings-and-version-1",
			`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    paramStr: val1
`,
			expectValid,
		},
		{"no-conversions",
			`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: module-one
spec:
  enabled: false
`,
			expectValid,
		},
		{
			"empty spec.settings with enabled:false",
			`apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: flant-integration
spec:
  version: 1
  enabled: false
`,
			expectValid,
		},

		// Invalid cases
		{
			"forbidden with oneOf",
			`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 2
  settings:
    tcpEnabled: false
    udpEnabled: false
`,
			expectInvalid,
		},
		{
			"empty spec.settings with enabled:true",
			`apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: flant-integration
spec:
  version: 1
  enabled: true
`,
			expectInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			vv := validation.NewValuesValidator()
			err := mock.AddOpenAPISchemas(vv, "global", "testdata/validator/global")
			g.Expect(err).ShouldNot(HaveOccurred(), "should load OpenAPI", tt.manifest)

			err = mock.AddOpenAPISchemas(vv, "flant-integration", "testdata/validator/flant-integration")
			g.Expect(err).ShouldNot(HaveOccurred(), "should load OpenAPI", tt.manifest)

			v := NewConfigValidator(vv)
			cfg, err := modCfgFromYAML(tt.manifest)
			g.Expect(err).ShouldNot(HaveOccurred(), "should parse manifest: %s", tt.manifest)

			res := v.Validate(cfg)

			switch tt.expect {
			case expectValid:
				g.Expect(res.HasError()).Should(BeFalse(), "should be valid, got error: %s", res.Error)
			case expectInvalid:
				g.Expect(res.HasError()).Should(BeTrue(), "should be invalid, got no error")
			}
		})
	}
}

func modCfgFromYAML(manifest string) (*v1alpha1.ModuleConfig, error) {
	var obj v1alpha1.ModuleConfig
	err := yaml.Unmarshal([]byte(manifest), &obj)
	if err != nil {
		return nil, fmt.Errorf("parsing manifest\n%s\n: %v", manifest, err)
	}

	return &obj, nil
}
