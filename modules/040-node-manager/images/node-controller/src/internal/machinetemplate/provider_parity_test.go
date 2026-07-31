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

package machinetemplate

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/deckhouse/node-controller/internal/controller/nodegroup/machineclass"
)

// The parity harness. A provider migrating to the v2 contract must prove two things, on the real
// files it ships:
//
//  1. Render parity — the v2 template renders byte-identical spec to the v1 one on the same
//     inputs. The migration is a change of contract, not of provider logic.
//  2. Rollout parity — the declarative rolloutFields answer "does this change recreate machines?"
//     exactly as the v1 instance-class.checksum did. Every deviation must be listed, with a
//     reason, in rolloutExceptions — an unlisted one fails the test.
//
// Together they are what lets us claim the migration rolls nobody's machines: the object the
// cluster gets is the same, and the rule that names it reacts to the same changes.
//
// The v1 side is read from testdata/v1/<provider>, a snapshot of the provider files taken at
// migration time. It is intentionally a copy: the real v1 files are deleted from the provider
// modules by the migration, and the guarantee "v2 behaves like the v1 that was in production"
// must outlive them.

const (
	parityClusterUUID = "aa11bb22-cc33-dd44-ee55-ff6677889900"
	parityPodSubnet   = "10.111.0.0/16"
	parityNodeGroup   = "worker"
	parityZone        = "zone-a"
)

type providerFixture struct {
	// name is the provider's cloud type, and also the testdata/v1 subdirectory.
	name string
	// contractPath points at the file the provider module really ships today.
	contractPath string
	// providerConfig is this provider's subtree of the d8-node-manager-cloud-provider secret.
	providerConfig map[string]any
	instanceClass  map[string]any
	// manualRolloutIDIgnoredByV1 records that this provider's v1 checksum template never read
	// manualRolloutID, so the operator's `manual-rollout-id` annotation did not roll its CAPI
	// machines at all. v2 honours the lever for every provider, in node-controller rather than in
	// each provider file; adoption stores the current id, so no migrating cluster rolls from it.
	manualRolloutIDIgnoredByV1 bool
}

func providerFixtures() []providerFixture {
	return []providerFixture{
		{
			name:           "dvp",
			contractPath:   "../../../../../../030-cloud-provider-dvp/capi/template.yaml",
			providerConfig: map[string]any{},
			instanceClass: map[string]any{
				"virtualMachine": map[string]any{
					"virtualMachineClassName": "generic-vm-class",
					"bootloader":              "EFI",
					"cpu":                     map[string]any{"cores": float64(4), "coreFraction": "50%"},
					"memory":                  map[string]any{"size": "8Gi"},
				},
				"rootDisk": map[string]any{
					"size":         "50Gi",
					"storageClass": "linstor-thin-r1",
					"image":        map[string]any{"kind": "ClusterVirtualImage", "name": "ubuntu-24-04"},
				},
				"additionalDisks": []any{
					map[string]any{"size": "10Gi", "storageClass": "linstor-thin-r2"},
				},
				"etcdDisk": map[string]any{"size": "20Gi", "storageClass": "linstor-thin-r1"},
			},
			manualRolloutIDIgnoredByV1: true,
		},
		{
			name:         "yandex",
			contractPath: "../../../../../../030-cloud-provider-yandex/capi/template.yaml",
			providerConfig: map[string]any{
				"instanceClassDefaults":       map[string]any{"imageID": "fd8default"},
				"zoneToSubnetIdMap":           map[string]any{parityZone: "e9bsubnet"},
				"sshKey":                      "ssh-ed25519 AAAA",
				"nodeNetworkCIDR":             "10.222.0.0/16",
				"shouldAssignPublicIPAddress": false,
			},
			instanceClass: map[string]any{
				"platformID":            "standard-v3",
				"cores":                 float64(4),
				"coreFraction":          float64(20),
				"memory":                float64(8192),
				"gpus":                  float64(0),
				"diskType":              "network-ssd",
				"diskSizeGB":            float64(80),
				"imageID":               "fd8image",
				"mainSubnet":            "e9bmain",
				"assignPublicIPAddress": true,
				"additionalSubnets":     []any{"e9badd1"},
				"preemptible":           false,
				"additionalLabels":      map[string]any{"env": "test"},
				"networkType":           "Standard",
			},
			// The one yandex divergence is value-specific, not field-specific, so it cannot be
			// expressed here — see TestYandexDefaultDiskSizeDivergence.
		},
		{
			name:         "openstack",
			contractPath: "../../../../../../../ee/modules/030-cloud-provider-openstack/capi/template.yaml",
			providerConfig: map[string]any{
				"instances": map[string]any{
					"imageName":          "ubuntu-24-04",
					"sshKeyPairName":     "deckhouse",
					"securityGroups":     []any{"default"},
					"mainNetwork":        "public",
					"additionalNetworks": []any{"internal"},
				},
				"podNetworkMode":       "DirectRoutingWithPortSecurityEnabled",
				"internalNetworkNames": []any{"internal"},
				"tags":                 map[string]any{"env": "test"},
			},
			instanceClass: map[string]any{
				"flavorName":               "m1.large",
				"imageName":                "ubuntu-24-04-ic",
				"mainNetwork":              "ic-main",
				"additionalNetworks":       []any{"ic-additional"},
				"rootDiskSize":             float64(50),
				"additionalSecurityGroups": []any{"ic-sg"},
				"additionalTags":           map[string]any{"team": "platform"},
				"capacity":                 map[string]any{"cores": float64(4), "memory": "8Gi"},
			},
		},
		{
			name:         "huaweicloud",
			contractPath: "../../../../../../../ee/modules/030-cloud-provider-huaweicloud/capi/template.yaml",
			providerConfig: map[string]any{
				"subnetId":        "subnet-default",
				"securityGroupId": "sg-default",
			},
			instanceClass: map[string]any{
				"imageName":          "ubuntu-24-04",
				"flavorName":         "s6.large.2",
				"rootDiskSize":       float64(40),
				"rootDiskType":       "SSD",
				"subnets":            []any{"subnet-extra"},
				"mainNetwork":        "subnet-main",
				"additionalNetworks": []any{"subnet-additional"},
				"securityGroups":     []any{"sg-extra"},
				"serverGroupID":      "sg-id",
				"vipAddress":         "10.0.0.7",
			},
			manualRolloutIDIgnoredByV1: true,
		},
		{
			name:           "dynamix",
			contractPath:   "../../../../../../../ee/modules/030-cloud-provider-dynamix/capi/template.yaml",
			providerConfig: map[string]any{},
			instanceClass: map[string]any{
				"imageName":       "ubuntu-24-04",
				"numCPUs":         float64(4),
				"memory":          float64(8192),
				"rootDiskSizeGb":  float64(40),
				"externalNetwork": "extnet",
			},
			manualRolloutIDIgnoredByV1: true,
		},
		{
			name:         "vcd",
			contractPath: "../../../../../../../ee/modules/030-cloud-provider-vcd/capi/template.yaml",
			providerConfig: map[string]any{
				"metadata": map[string]any{"owner": "platform"},
			},
			instanceClass: map[string]any{
				"storageProfile":     "vSAN Default",
				"template":           "org/catalog/ubuntu-24-04",
				"rootDiskSizeGb":     float64(40),
				"sizingPolicy":       "4cpu-8ram",
				"placementPolicy":    "rack-1",
				"additionalMetadata": map[string]any{"team": "platform"},
			},
		},
		{
			name:           "zvirt",
			contractPath:   "../../../../../../../ee/se-plus/modules/030-cloud-provider-zvirt/capi/template.yaml",
			providerConfig: map[string]any{},
			instanceClass: map[string]any{
				"template":       "ubuntu-24-04",
				"vnicProfileID":  "vnic-1",
				"numCPUs":        float64(4),
				"memory":         float64(8192),
				"rootDiskSizeGb": float64(40),
			},
			manualRolloutIDIgnoredByV1: true,
		},
	}
}

func (f providerFixture) legacyPath(file string) string {
	return filepath.Join("testdata", "v1", f.name, file)
}

// TestProviderRenderParity renders every migrated provider through both engines and requires the
// resulting objects to be identical.
func TestProviderRenderParity(t *testing.T) {
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			contract := loadContract(t, fixture.contractPath)

			v2Object, err := Render(contract, RenderContext{
				InstanceClass: fixture.instanceClass,
				Provider:      fixture.providerConfig,
				Zone:          parityZone,
				NodeGroupName: parityNodeGroup,
				ClusterUUID:   parityClusterUUID,
				PodSubnet:     parityPodSubnet,
			})
			require.NoError(t, err, "v2 template must render")

			v1Object := renderLegacyTemplate(t, fixture)

			assert.Equal(t, v1Object, v2Object,
				"v2 must render exactly what v1 rendered: a different object here means the migration "+
					"changes machines, not just the contract")
		})
	}
}

// TestProviderRolloutParity mutates every field of the fixture InstanceClass and requires the v2
// rolloutFields decision to match the v1 checksum decision.
func TestProviderRolloutParity(t *testing.T) {
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			contract := loadContract(t, fixture.contractPath)
			checksumTemplate, err := os.ReadFile(fixture.legacyPath("instance-class.checksum"))
			require.NoError(t, err)

			baseChecksum := renderLegacyChecksum(t, fixture, checksumTemplate, fixture.instanceClass, "")

			for _, path := range mutationPaths(fixture, contract) {
				t.Run(path, func(t *testing.T) {
					mutated := mutateSpec(t, fixture.instanceClass, path)

					v1Rolls := renderLegacyChecksum(t, fixture, checksumTemplate, mutated, "") != baseChecksum

					changes, err := Changes(fixture.instanceClass, mutated, contract.RolloutFields)
					require.NoError(t, err)
					v2Rolls := len(changes) > 0

					assert.Equal(t, v1Rolls, v2Rolls,
						"changing %s: v1 checksum rolls=%v, v2 rolloutFields rolls=%v — either fix "+
							"rolloutFields or document the difference in rolloutExceptions",
						path, v1Rolls, v2Rolls)
				})
			}
		})
	}
}

// TestProviderManualRolloutIDParity checks the operator's explicit lever separately: it does not
// live in the InstanceClass, so the field-mutation loop cannot reach it.
func TestProviderManualRolloutIDParity(t *testing.T) {
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			checksumTemplate, err := os.ReadFile(fixture.legacyPath("instance-class.checksum"))
			require.NoError(t, err)

			base := renderLegacyChecksum(t, fixture, checksumTemplate, fixture.instanceClass, "")
			rolled := renderLegacyChecksum(t, fixture, checksumTemplate, fixture.instanceClass, "2026-07-31")
			v1Rolls := base != rolled

			// v2 compares the stored manual-rollout-id for every provider, in node-controller
			// rather than in the provider file — see ensureMachineTemplateGeneration. So the only
			// possible difference is a provider whose v1 checksum ignored the annotation, and each
			// such provider has to say so in its fixture.
			assert.Equal(t, !fixture.manualRolloutIDIgnoredByV1, v1Rolls,
				"the v1 behaviour of manual-rollout-id changed: update the fixture or the migration notes")
		})
	}
}

// TestYandexDefaultDiskSizeDivergence pins the single known v1/v2 disagreement, which is about a
// value rather than a field: the v1 yandex checksum dropped diskSizeGB when it equalled the
// default 50, so writing an explicit `diskSizeGB: 50` into an InstanceClass that had none did not
// roll. v2 compares values as they are, so it does.
//
// It cannot roll an existing cluster: on the first v2 reconcile node-controller adopts the live
// template and stores the spec as it is, so nothing differs until the user really edits the field.
func TestYandexDefaultDiskSizeDivergence(t *testing.T) {
	fixture := fixtureByName(t, "yandex")

	contract := loadContract(t, fixture.contractPath)
	checksumTemplate, err := os.ReadFile(fixture.legacyPath("instance-class.checksum"))
	require.NoError(t, err)

	withoutDiskSize := deepCopySpec(t, fixture.instanceClass)
	delete(withoutDiskSize, "diskSizeGB")
	withDefaultDiskSize := deepCopySpec(t, withoutDiskSize)
	withDefaultDiskSize["diskSizeGB"] = float64(50)

	v1Base := renderLegacyChecksum(t, fixture, checksumTemplate, withoutDiskSize, "")
	v1Default := renderLegacyChecksum(t, fixture, checksumTemplate, withDefaultDiskSize, "")
	assert.Equal(t, v1Base, v1Default, "v1 ignored an explicit default disk size")

	changes, err := Changes(withoutDiskSize, withDefaultDiskSize, contract.RolloutFields)
	require.NoError(t, err)
	assert.Len(t, changes, 1, "v2 sees an added field as a change")
	assert.Equal(t, "diskSizeGB", changes[0].Path)
}

func fixtureByName(t *testing.T, name string) providerFixture {
	t.Helper()
	for _, fixture := range providerFixtures() {
		if fixture.name == name {
			return fixture
		}
	}
	t.Fatalf("no provider fixture named %q", name)
	return providerFixture{}
}

func loadContract(t *testing.T, path string) *Contract {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "provider must ship the v2 contract file")
	contract, err := ParseContract(data)
	require.NoError(t, err, "provider contract must be valid")
	return contract
}

// renderLegacyTemplate renders the v1 machine-template.yaml in the v1 context (the synthetic helm
// values tree) and strips the metadata block v1 templates carried themselves — under v2
// node-controller stamps it.
func renderLegacyTemplate(t *testing.T, fixture providerFixture) map[string]any {
	t.Helper()

	templateContent, err := os.ReadFile(fixture.legacyPath("machine-template.yaml"))
	require.NoError(t, err)

	nodeGroupValues := legacyNodeGroupValues(fixture.instanceClass, "")
	renderCtx := map[string]any{
		"Values": map[string]any{
			"global": map[string]any{
				"discovery": map[string]any{
					"clusterUUID": parityClusterUUID,
					"podSubnet":   parityPodSubnet,
				},
			},
			"nodeManager": map[string]any{
				"internal": map[string]any{
					"cloudProvider": map[string]any{fixture.name: fixture.providerConfig},
				},
			},
		},
		"nodeGroup":             nodeGroupValues,
		"zoneName":              parityZone,
		"templateName":          parityNodeGroup + "-8ad9c341",
		"instanceClassChecksum": "3f7a02c1",
	}

	rendered, err := machineclass.RenderMachineClass(templateContent, renderCtx)
	require.NoError(t, err, "v1 template must still render (it is the reference)")

	obj := map[string]any{}
	require.NoError(t, sigsyaml.Unmarshal(rendered, &obj))
	delete(obj, "metadata")
	return obj
}

func renderLegacyChecksum(t *testing.T, fixture providerFixture, template []byte, instanceClass map[string]any, rolloutID string) string {
	t.Helper()
	cloudProvider := map[string]any{fixture.name: fixture.providerConfig}
	checksum, err := machineclass.RenderChecksum(template, legacyNodeGroupValues(instanceClass, rolloutID), cloudProvider)
	require.NoError(t, err)
	return checksum
}

func legacyNodeGroupValues(instanceClass map[string]any, rolloutID string) map[string]any {
	return map[string]any{
		"name":            parityNodeGroup,
		"instanceClass":   instanceClass,
		"manualRolloutID": rolloutID,
		"cloudInstances": map[string]any{
			"classReference": map[string]any{"name": parityNodeGroup + "-class"},
		},
	}
}

// mutationPaths is every leaf of the fixture InstanceClass plus every rolloutField the fixture
// does not set — so the harness covers both "value changed" and "field appeared".
func mutationPaths(fixture providerFixture, contract *Contract) []string {
	paths := map[string]struct{}{}
	collectLeafPaths(fixture.instanceClass, "", paths)
	for _, field := range contract.RolloutFields {
		paths[field] = struct{}{}
	}

	out := make([]string, 0, len(paths))
	for path := range paths {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func collectLeafPaths(value map[string]any, prefix string, out map[string]struct{}) {
	for key, val := range value {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if nested, ok := val.(map[string]any); ok && len(nested) > 0 {
			collectLeafPaths(nested, path, out)
			continue
		}
		out[path] = struct{}{}
	}
}

// mutateSpec returns a deep copy of the spec with one path changed to a value that is different
// but still type-plausible, so provider templates keep rendering.
func mutateSpec(t *testing.T, spec map[string]any, path string) map[string]any {
	t.Helper()

	mutated := deepCopySpec(t, spec)
	segments := strings.Split(path, ".")
	current := mutated
	for _, segment := range segments[:len(segments)-1] {
		next, ok := current[segment].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[segment] = next
		}
		current = next
	}
	last := segments[len(segments)-1]
	current[last] = mutatedValue(current[last])
	return mutated
}

func mutatedValue(value any) any {
	switch typed := value.(type) {
	case string:
		return typed + "-changed"
	case float64:
		return typed + 1
	case bool:
		return !typed
	case []any:
		return append(append([]any{}, typed...), "extra")
	case map[string]any:
		out := map[string]any{"extra": "value"}
		for k, v := range typed {
			out[k] = v
		}
		return out
	default:
		// absent field: give it a value, which is the "field appeared" case
		return "value"
	}
}

func deepCopySpec(t *testing.T, spec map[string]any) map[string]any {
	t.Helper()
	return runtime.DeepCopyJSON(spec)
}
