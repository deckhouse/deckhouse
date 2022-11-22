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

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/values/validation"
	"github.com/go-openapi/spec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// TODO (future) make this test more generic to be available for all modules with CRDs.

var (
	validCases = map[string]string{
		"settings-and-version-1": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    paramStr: val1
`,
		"settings-and-version-2": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 2
  settings:
    paramStr: val1
`,
		"settings-versions-enabled": `
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
		"enabled-only": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  enabled: true
`,
		"empty": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec: {}
`,
	}

	invalidCases = map[string]string{
		"settings-and-version-0": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 0
  settings:
    paramStr: val1
		`,
		"settings-only": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings:
    paramStr: val1
`,
		"version-only": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
`,
		"settings-and-enabled": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings:
    paramStr: val1
  enabled: false
`,
		"version-and-enabled": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  enabledff: false
`,
	}
)

var _ = Describe("Module :: deckhouse-config :: ModuleConfig CRs ::", func() {
	s, loadErr := crdValidator(moduleConfigCRDPath)

	Context("schema loader", func() {
		It("should load schema", func() {
			Expect(loadErr).ShouldNot(HaveOccurred())
			Expect(s).ShouldNot(BeNil())
		})
	})

	Context("giving valid ModuleConfig", func() {
		for testName, manifest := range validCases {
			validManifest := manifest
			It(fmt.Sprintf("should validate %s", testName), func() {
				err := validateManifest(validManifest, s)
				Expect(err).ShouldNot(HaveOccurred(), "should validate %s", validManifest)
			})
		}
	})

	Context("giving invalid ModulesConfig", func() {
		for testName, manifest := range invalidCases {
			invalidManifest := manifest
			It(fmt.Sprintf("should not validate %s", testName), func() {
				err := validateManifest(invalidManifest, s)
				Expect(err).Should(HaveOccurred(), "should not validate %s", invalidManifest)
			})
		}
	})
})

func crdValidator(crdYamlPath string) (*spec.Schema, error) {
	_ = fmt.Sprintf(`
type: object
properties:
  apiVersion:
    type: string
    enum: ["deckhouse.io/v1alpha1"]
  kind:
    type: string
    enum: [ModuleConfig]
  metadata:
    type: object
    required: [name]
    properties:
      name:
        type: string
      namespace:
        type: string
  spec:
    type: object
    properties:
      enabled:
        type: boolean
        description: |
          Enables or disables a module.
        example: 'false'
      version:
        type: number
        description: |
          Version of settings schema.
        example: '1'
        minimum: 1
      settings:
        type: object
        description: |
          Module settings.
        x-kubernetes-preserve-unknown-fields: true
    oneOf:
    - minProperties: 0
      maxProperties: 0
    - required: [enabled]
      minProperties: 1
      maxProperties: 1
    - required: [version, settings]
      minProperties: 2
      maxProperties: 2
    - required: [version, settings, enabled]

    #$ref: '%s#/spec/versions/0/schema/openAPIV3Schema/properties/spec'
`, crdYamlPath)

	crdSchema := fmt.Sprintf(`
type: object
properties:
  apiVersion:
    type: string
    enum: ["deckhouse.io/v1alpha1"]
  kind:
    type: string
    enum: [ModuleConfig]
  metadata:
    type: object
    required: [name]
    properties:
      name:
        type: string
      namespace:
        type: string
  spec:
    $ref: '%s#/spec/versions/0/schema/openAPIV3Schema/properties/spec'
`, crdYamlPath)

	return validation.LoadSchemaFromBytes([]byte(crdSchema))
}

func validateManifest(yamlManifest string, s *spec.Schema) (multiErr error) {
	if s == nil {
		return fmt.Errorf("validate config: schema is not provided")
	}

	var obj unstructured.Unstructured
	err := yaml.Unmarshal([]byte(yamlManifest), &obj)
	if err != nil {
		return fmt.Errorf("parsing manifest\n%s\n: %v", yamlManifest, err)
	}

	return validation.ValidateObject(obj.UnstructuredContent(), s, "")
}
