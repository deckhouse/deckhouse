/*
Copyright 2026 Flant JSC

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

package cloudprovider

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sigsyaml "sigs.k8s.io/yaml"
)

// repoRoot is relative to this package's directory, which is where `go test` runs:
// <root>/modules/040-node-manager/images/node-controller/src/internal/cloudprovider.
const repoRoot = "../../../../../../.."

// instanceClassCRDPaths are the two places a provider module keeps its InstanceClass CRD,
// relative to the module directory: hand-written providers put it in candi, DVP generates it.
var instanceClassCRDPaths = []string{
	"candi/openapi/instance_class.yaml",
	"crds/instance_class.yaml",
}

var publishedAPIVersionRe = regexp.MustCompile(InstanceClassAPIVersionKey + `:\s*{{\s*b64enc\s+"([^"]+)"`)
var providerTypeRe = regexp.MustCompile(`(?m)^type:\s*{{\s*b64enc\s+"([^"]+)"`)
var machineClassKindRe = regexp.MustCompile(`(?m)^machineClassKind:\s*{{\s*b64enc\s+"([^"]*)"`)

var commonRegistrationKeys = []string{
	"type", "region", "zones", "instanceClassKind", InstanceClassAPIVersionKey,
}

var capiRegistrationKeys = []string{
	"capiClusterName", "capiClusterKind", "capiClusterAPIVersion",
	"capiMachineTemplateKind", "capiMachineTemplateAPIVersion",
}

// A provider that publishes the wrong version, or none, is not visible in review — it simply stops
// rendering. Changing the version also moves the instance-class checksum, and the checksum names
// an immutable MachineTemplate whose rename recreates every node in the NodeGroup. The registered
// version is allowed to differ from the storage version, but it must remain served by the CRD.
func TestEveryCloudProviderPublishesValidRegistrationContract(t *testing.T) {
	// Every tree a cloud provider module can live in.
	registrationGlobs := []string{
		"modules/030-cloud-provider-*/templates/registration.yaml",
		"ee/modules/030-cloud-provider-*/templates/registration.yaml",
		"ee/se-plus/modules/030-cloud-provider-*/templates/registration.yaml",
	}

	var paths []string
	for _, glob := range registrationGlobs {
		matches, err := filepath.Glob(filepath.Join(repoRoot, glob))
		require.NoError(t, err)
		paths = append(paths, matches...)
	}
	if len(paths) == 0 {
		t.Skip("cloud provider modules are not reachable from here; nothing to check")
	}

	for _, path := range paths {
		moduleDir := filepath.Dir(filepath.Dir(path))
		t.Run(filepath.Base(moduleDir), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			content := string(raw)
			for _, key := range commonRegistrationKeys {
				require.Regexp(t, regexp.MustCompile(`(?m)^`+regexp.QuoteMeta(key)+`:`), content,
					"%s must publish required registration key %s", path, key)
			}
			typeMatch := providerTypeRe.FindStringSubmatch(content)
			require.NotNil(t, typeMatch, "%s must publish a literal provider type", path)
			require.Regexp(t, regexp.MustCompile(`(?m)^`+regexp.QuoteMeta(typeMatch[1])+`:`), content,
				"%s must publish the provider-owned %s subtree", path, typeMatch[1])

			hasCAPI := strings.Contains(content, "capiClusterKind:")
			if hasCAPI {
				for _, key := range capiRegistrationKeys {
					require.Regexp(t, regexp.MustCompile(`(?m)^`+regexp.QuoteMeta(key)+`:`), content,
						"%s must publish the complete CAPI registration group; missing %s", path, key)
				}
			}
			machineClassMatch := machineClassKindRe.FindStringSubmatch(content)
			hasMCM := machineClassMatch != nil && machineClassMatch[1] != ""
			require.True(t, hasCAPI || hasMCM,
				"%s must enable at least one of CAPI or MCM", path)

			if !strings.Contains(content, "instanceClassKind:") {
				t.Skip("provider does not publish an InstanceClass kind")
			}

			match := publishedAPIVersionRe.FindStringSubmatch(content)
			require.NotNil(t, match,
				"%s publishes instanceClassKind, so it must publish %s next to it",
				path, InstanceClassAPIVersionKey)

			require.True(t, instanceClassCRDServesVersion(t, moduleDir, match[1]),
				"%s publishes InstanceClass version %q, but the CRD does not serve it", path, match[1])

			// node-controller finds registrations by this label, not by the Secret name; both
			// rendered Secrets (the legacy fixed-name one and the per-provider one) must carry it.
			require.GreaterOrEqual(t, strings.Count(content, RegistrationSecretLabel), 2,
				"%s must label both registration Secrets with %s", path, RegistrationSecretLabel)
		})
	}
}

func instanceClassCRDServesVersion(t *testing.T, moduleDir, wantedVersion string) bool {
	t.Helper()

	var crd struct {
		Spec struct {
			Versions []struct {
				Name   string `json:"name"`
				Served bool   `json:"served"`
			} `json:"versions"`
		} `json:"spec"`
	}

	for _, rel := range instanceClassCRDPaths {
		raw, err := os.ReadFile(filepath.Join(moduleDir, rel))
		if os.IsNotExist(err) {
			continue
		}
		require.NoError(t, err)
		require.NoError(t, sigsyaml.Unmarshal(raw, &crd))

		for _, version := range crd.Spec.Versions {
			if version.Name == wantedVersion {
				return version.Served
			}
		}
		return false
	}

	require.Fail(t, "no InstanceClass CRD", "%s has none of %v", moduleDir, instanceClassCRDPaths)
	return false
}

func TestDecodeRegistrationAcceptsProviderEncodings(t *testing.T) {
	registration, err := DecodeRegistration(map[string][]byte{
		"type":                           []byte("openstack"),
		"region":                         []byte(`"RegionOne"`),
		"zones":                          []byte(`["zone-a","zone-b"]`),
		"instanceClassKind":              []byte("OpenStackInstanceClass"),
		"instanceClassAPIVersion":        []byte(`"v1"`),
		"machineClassKind":               []byte("OpenStackMachineClass"),
		"sshPublicKey":                   []byte("ssh-ed25519 AAAA"),
		"capiClusterName":                []byte("openstack"),
		"capiClusterKind":                []byte(`"OpenStackCluster"`),
		"capiClusterAPIVersion":          []byte("infrastructure.cluster.x-k8s.io/v1beta1"),
		"capiMachineTemplateKind":        []byte("OpenStackMachineTemplate"),
		"capiMachineTemplateAPIVersion":  []byte("infrastructure.cluster.x-k8s.io/v1beta1"),
		"capiMachineDeploymentSpecPatch": []byte("spec: {}"),
		"openstack":                      []byte(`{"connection":{"region":"RegionOne"}}`),
	})
	require.NoError(t, err)

	assert.Equal(t, "openstack", registration.Type)
	assert.Equal(t, "RegionOne", registration.Region)
	assert.Equal(t, []string{"zone-a", "zone-b"}, registration.Zones)
	assert.Equal(t, "OpenStackInstanceClass", registration.InstanceClassKind)
	assert.Equal(t, "v1", registration.InstanceClassAPIVersion)
	assert.Equal(t, "OpenStackMachineClass", registration.MachineClassKind)
	assert.Equal(t, "ssh-ed25519 AAAA", registration.SSHPublicKey)
	assert.Equal(t, "openstack", registration.CAPIClusterName)
	assert.Equal(t, "OpenStackCluster", registration.CAPIClusterKind)
	assert.Equal(t, "infrastructure.cluster.x-k8s.io/v1beta1", registration.CAPIClusterAPIVersion)
	assert.Equal(t, "OpenStackMachineTemplate", registration.CAPIMachineTemplateKind)
	assert.Equal(t, "infrastructure.cluster.x-k8s.io/v1beta1", registration.CAPIMachineTemplateAPIVersion)
	assert.Equal(t, "spec: {}", registration.CAPIMachineDeploymentSpecPatch)

	connection, ok := registration.CloudVariables["connection"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "RegionOne", connection["region"])
}

func TestDecodeRegistrationWithoutProviderSubtreeUsesEmptyMap(t *testing.T) {
	registration, err := DecodeRegistration(map[string][]byte{"type": []byte("dvp")})
	require.NoError(t, err)
	assert.Nil(t, registration.CloudVariables)
}

func TestDecodeRegistrationRejectsMalformedJSON(t *testing.T) {
	_, err := DecodeRegistration(map[string][]byte{"zones": []byte("not-json")})
	require.ErrorContains(t, err, "zones")

	_, err = DecodeRegistration(map[string][]byte{
		"type": []byte("dvp"),
		"dvp":  []byte("not-json"),
	})
	require.ErrorContains(t, err, "dvp subtree")
}

func TestRegistrationValidation(t *testing.T) {
	registration := Registration{
		Type: "dvp", Region: "default", Zones: []string{"default"},
		InstanceClassKind: "DVPInstanceClass", InstanceClassAPIVersion: "v1alpha1",
		CAPIClusterName: "dvp", CAPIClusterKind: "DeckhouseCluster",
		CAPIClusterAPIVersion:         "infrastructure.cluster.x-k8s.io/v1alpha1",
		CAPIMachineTemplateKind:       "DeckhouseMachineTemplate",
		CAPIMachineTemplateAPIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
		CloudVariables:                map[string]any{"project": "test"},
	}
	require.NoError(t, registration.ValidateCore())
	require.NoError(t, registration.ValidateCAPI())

	registration.Region = ""
	require.ErrorContains(t, registration.ValidateCore(), "region")
	registration.Region = "default"
	registration.CAPIMachineTemplateKind = ""
	require.ErrorContains(t, registration.ValidateCAPI(), "capiMachineTemplateKind")
	registration.CAPIMachineTemplateKind = "DeckhouseMachineTemplate"
	registration.Zones = []string{""}
	require.ErrorContains(t, registration.ValidateCore(), "zones")
	registration.Zones = []string{"default"}
	registration.CloudVariables = map[string]any{}
	require.NoError(t, registration.ValidateCore(), "an empty provider object is valid for providers without settings")
	registration.Type = "DVP"
	require.ErrorContains(t, registration.ValidateCore(), "must be lowercase")
}
