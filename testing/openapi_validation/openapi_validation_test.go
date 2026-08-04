//go:build validation
// +build validation

/*
Copyright 2021 Flant JSC

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

package openapi_validation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

var ignoredEnum = []string{
	"properties.license.properties.edition",
	// Real Kubernetes ClusterRole names ("cluster-admin", "user-authz:cluster-admin"); set by the
	// reconcile_kubeadm_cluster_admins_binding hook in 040-control-plane-manager. RBAC ClusterRole
	// names are lowercase by convention and cannot be capitalized.
	"properties.internal.properties.kubeadmClusterAdminsTargetRoleName",
}

func TestValidationOpenAPI(t *testing.T) {
	apiFiles, err := GetOpenAPIYAMLFiles(deckhousePath)
	require.NoError(t, err)

	filesC := make(chan fileValidation, len(apiFiles))
	resultC := RunOpenAPIValidator(filesC)

	for _, apiFile := range apiFiles {
		filesC <- fileValidation{
			filePath: apiFile,
		}
	}
	close(filesC)

	// TODO: is it necessary to check for dmt checked values?
	for result := range resultC {
		if strings.Contains(result.validationError.Error(), "Enum") {
			// skip enum validation for ignored properties
			var isIgnored bool
			for _, ignored := range ignoredEnum {
				if strings.Contains(result.validationError.Error(), ignored) {
					isIgnored = true
					break
				}
			}
			if isIgnored {
				continue
			}
		}
		assert.NoError(t, result.validationError, "File '%s' has invalid spec", strings.TrimPrefix(result.filePath, deckhousePath))
	}
}

// TestValidators test that validation hooks are working
func TestValidators(t *testing.T) {
	apiFiles := []string{deckhousePath + "testing/openapi_validation/openapi_testdata/values.yaml"}

	filesC := make(chan fileValidation, len(apiFiles))
	resultC := RunOpenAPIValidator(filesC)

	for _, apiFile := range apiFiles {
		filesC <- fileValidation{
			filePath: apiFile,
		}
	}
	close(filesC)

	for res := range resultC {
		assert.Error(t, res.validationError)
		err, ok := res.validationError.(*multierror.Error)
		require.True(t, ok)
		require.Len(t, err.Errors, 6)

		// we can't guarantee order here, thats why test contains
		assert.Contains(t, res.validationError.Error(), "properties.https is invalid: must have no default value")
		assert.Contains(t, res.validationError.Error(), "Enum 'properties.https.properties.mode.enum' is invalid: value 'disabled' must start with Capital letter")
		assert.Contains(t, res.validationError.Error(), "Enum 'properties.https.properties.mode.enum' is invalid: value: 'Cert-Manager' must be in CamelCase")
		assert.Contains(t, res.validationError.Error(), "Enum 'properties.https.properties.mode.enum' is invalid: value: 'Some:Thing' must be in CamelCase")
		assert.Contains(t, res.validationError.Error(), "Enum 'properties.https.properties.mode.enum' is invalid: value: 'Any.Thing' must be in CamelCase")
		assert.Contains(t, res.validationError.Error(), "properties.highAvailability is invalid: must have no default value")
	}
}

func TestCRDValidators(t *testing.T) {
	apiFiles := []string{deckhousePath + "testing/openapi_validation/openapi_testdata/crd.yaml"}

	filesC := make(chan fileValidation, len(apiFiles))
	resultC := RunOpenAPIValidator(filesC)

	for _, apiFile := range apiFiles {
		filesC <- fileValidation{
			filePath: apiFile,
		}
	}
	close(filesC)

	for res := range resultC {
		assert.Error(t, res.validationError)
		err, ok := res.validationError.(*multierror.Error)
		require.True(t, ok)
		require.Len(t, err.Errors, 1)

		// we can't guarantee order here, thats why test contains
		assert.Contains(t, res.validationError.Error(), "file validation error: wrong property")
	}
}

func TestModulesVersionsValidation(t *testing.T) {
	mv, err := modulesVersions(deckhousePath)
	require.NoError(t, err)
	for m, v := range mv {
		message := fmt.Sprintf("conversions version(%d) and spec version(%d) for module %s are not equal",
			v.conversionsVersion, v.specVersion, m)
		assert.Equal(t, true, v.conversionsVersion == v.specVersion, message)
	}
}

// kubernetesVersionEditions ties every schema that offers a kubernetesVersion choice to the
// edition it belongs to. ee/ ships no candi of its own and reuses the default one.
var kubernetesVersionEditions = []struct {
	name                 string
	versionMap           string
	clusterConfiguration string
	moduleConfigs        []string
}{
	{
		name:                 "default",
		versionMap:           "candi/version_map.yml",
		clusterConfiguration: "candi/openapi/cluster_configuration.yaml",
		moduleConfigs: []string{
			"modules/040-control-plane-manager/openapi/config-values.yaml",
			"ee/modules/040-control-plane-manager/openapi/config-values.yaml",
		},
	},
	{
		name:                 "cse",
		versionMap:           "ee/cse/candi/version_map.yml",
		clusterConfiguration: "ee/cse/candi/openapi/cluster_configuration.yaml",
		moduleConfigs: []string{
			"ee/cse/modules/040-control-plane-manager/openapi/config-values.yaml",
		},
	},
}

type kubernetesVersionEnum struct {
	Enum []string `yaml:"enum"`
}

type clusterConfigurationSchema struct {
	APIVersions []struct {
		OpenAPISpec struct {
			Properties struct {
				KubernetesVersion kubernetesVersionEnum `yaml:"kubernetesVersion"`
			} `yaml:"properties"`
		} `yaml:"openAPISpec"`
	} `yaml:"apiVersions"`
}

type moduleConfigValuesSchema struct {
	Properties struct {
		KubernetesVersion kubernetesVersionEnum `yaml:"kubernetesVersion"`
	} `yaml:"properties"`
}

type k8sVersionMap struct {
	K8s map[string]interface{} `yaml:"k8s"`
}

// readYAML loads relPath if it is present in this checkout, reporting whether it was found.
//
// Absence is not a failure: the CE test image ships no ee/ directory at all, so the EE and CSE
// schemas simply do not exist there. Failing on that would make the test un-runnable in CE — and
// it is the caller's job (see requireAnythingChecked) to ensure the whole test does not silently
// degrade into checking nothing.
func readYAML(t *testing.T, relPath string, out interface{}) bool {
	t.Helper()

	roots := []string{deckhousePath}
	if wd, err := os.Getwd(); err == nil {
		// go test runs with cwd = this package dir (testing/openapi_validation).
		roots = append(roots, filepath.Clean(filepath.Join(wd, "../..")), wd)
	}

	for _, root := range roots {
		path := filepath.Join(root, relPath)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		require.NoError(t, yaml.Unmarshal(data, out), "unmarshal %s", path)
		return true
	}
	return false
}

// TestKubernetesVersionEnumValidation keeps the kubernetesVersion pin lists in sync.
//
// ModuleConfig adds the Default sentinel (Automatic remains a deprecated alias). ClusterConfiguration
// still only offers Automatic. Pin versions (numeric) must match across CC and MC for each edition,
// and every pin must exist in that edition's version_map.
//
// TODO(kubernetesVersion-deprecation): T+1 rewrite — after CC field removal use ModuleConfig
// enum as reference vs edition version_map; drop Automatic alias checks; do not delete this test.
func TestKubernetesVersionEnumValidation(t *testing.T) {
	sentinelsCC := map[string]struct{}{"Automatic": {}}
	sentinelsMC := map[string]struct{}{"Automatic": {}, "Default": {}}

	// Guards against the test quietly becoming a no-op: it was inert for a while because its name
	// did not match the CI -run filter, and skipping absent editions must not recreate that.
	checkedSchemas := 0

	for _, edition := range kubernetesVersionEditions {
		t.Run(edition.name, func(t *testing.T) {
			var cc clusterConfigurationSchema
			if !readYAML(t, edition.clusterConfiguration, &cc) {
				t.Skipf("%s is absent in this checkout (edition not shipped here)", edition.clusterConfiguration)
			}
			require.NotEmpty(t, cc.APIVersions, "%s has no apiVersions", edition.clusterConfiguration)

			ccEnum := cc.APIVersions[0].OpenAPISpec.Properties.KubernetesVersion.Enum
			require.NotEmpty(t, ccEnum, "%s: kubernetesVersion has no enum", edition.clusterConfiguration)
			require.Contains(t, ccEnum, "Automatic")

			ccPins := pinVersions(ccEnum, sentinelsCC)

			for _, mcPath := range edition.moduleConfigs {
				var mc moduleConfigValuesSchema
				if !readYAML(t, mcPath, &mc) {
					continue
				}

				mcEnum := mc.Properties.KubernetesVersion.Enum
				require.Contains(t, mcEnum, "Default", "%s must offer Default", mcPath)
				require.Contains(t, mcEnum, "Automatic", "%s must keep Automatic alias", mcPath)
				assert.Equal(t, ccPins, pinVersions(mcEnum, sentinelsMC),
					"pinned kubernetesVersion values in %s differ from %s", mcPath, edition.clusterConfiguration)
				checkedSchemas++
			}

			var vm k8sVersionMap
			if readYAML(t, edition.versionMap, &vm) {
				require.NotEmpty(t, vm.K8s, "%s has no k8s section", edition.versionMap)
				for _, version := range ccPins {
					assert.Contains(t, vm.K8s, version,
						"version %q is offered by %s but absent from %s", version, edition.clusterConfiguration, edition.versionMap)
				}
			}
		})
	}

	require.Positive(t, checkedSchemas,
		"no ModuleConfig kubernetesVersion schema was checked in this checkout — the enum guard is inert")
}

func pinVersions(enum []string, sentinels map[string]struct{}) []string {
	out := make([]string, 0, len(enum))
	for _, v := range enum {
		if _, skip := sentinels[v]; skip {
			continue
		}
		out = append(out, v)
	}
	return out
}
