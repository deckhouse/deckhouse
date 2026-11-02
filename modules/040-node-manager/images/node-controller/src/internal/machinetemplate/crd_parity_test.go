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
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sigsyaml "sigs.k8s.io/yaml"
)

// The hand-written fixtures in provider_parity_test.go only cover the fields somebody thought to
// put in them. This harness takes the field list from the provider's own InstanceClass CRD
// instead, so a field nobody remembered — and therefore nobody put in rolloutFields — still gets
// asked the only question that matters:
//
//	does changing it roll machines under v2 exactly when it rolled them under v1?
//
// A field the provider forgot in rolloutFields fails here as "v1 rolls, v2 does not" (a change the
// user asks for silently not reaching the machines), and a field listed by mistake fails as "v2
// rolls, v1 does not" (a rollout the user did not ask for). Both are the failure modes this whole
// migration exists to avoid.

type crdField struct {
	path string
	kind string
}

func TestProviderRolloutParityAgainstCRD(t *testing.T) {
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			contract := loadContract(t, fixture.contractPath)
			checksumTemplate, err := os.ReadFile(fixture.legacyPath("instance-class.checksum"))
			require.NoError(t, err)

			fields := crdSpecFields(t, fixture.crdPath)
			require.NotEmpty(t, fields, "the provider CRD must describe its InstanceClass spec")

			baseChecksum := renderLegacyChecksum(t, fixture, checksumTemplate, fixture.instanceClass, "")

			// Three mutations per field, because the ways a provider file can drift differ:
			// a real new value, an empty one (the shape a GitOps template collapses to), and the
			// field disappearing altogether.
			mutations := []struct {
				name  string
				apply func(spec map[string]any, field crdField)
			}{
				{name: "set", apply: func(spec map[string]any, field crdField) {
					setPath(spec, field.path, crdSampleValue(field.kind))
				}},
				{name: "empty", apply: func(spec map[string]any, field crdField) {
					setPath(spec, field.path, crdZeroValue(field.kind))
				}},
				{name: "removed", apply: func(spec map[string]any, field crdField) {
					deletePath(spec, field.path)
				}},
			}

			for _, field := range fields {
				for _, mutation := range mutations {
					t.Run(field.path+"/"+mutation.name, func(t *testing.T) {
						if reason, documented := fixture.rolloutExceptions[field.path+"/"+mutation.name]; documented {
							t.Skip(reason)
						}
						if reason, excluded := fixture.postV1Reason(instanceClassNamespace, field.path); excluded {
							t.Skip(reason)
						}

						mutated := deepCopySpec(t, fixture.instanceClass)
						mutation.apply(mutated, field)

						v1Rolls := renderLegacyChecksum(t, fixture, checksumTemplate, mutated, "") != baseChecksum

						changes, err := Changes(fixture.instanceClass, mutated, contract.RolloutFields)
						require.NoError(t, err)
						v2Rolls := len(changes) > 0

						assert.Equal(t, v1Rolls, v2Rolls,
							"%s %s (declared in the provider CRD): v1 checksum rolls=%v, rolloutFields roll=%v — "+
								"either the field is missing from rolloutFields, or it is listed there while v1 ignored it",
							mutation.name, field.path, v1Rolls, v2Rolls)
					})
				}
			}
		})
	}
}

// TestNoRolloutMeansNoChangeToTheObject is the invariant that matters most, and it does not appeal
// to v1 at all: whenever node-controller decides NOT to create a generation, the object it would
// render must be identical to the one already in the cluster.
//
// Break it and a user's edit is silently dropped — the machines keep running a configuration the
// InstanceClass no longer describes, and nothing says so. It covers exactly what the v1-parity
// harness cannot: `rolloutFields` that misses a field the template actually reads, and the
// empty-value folding in fieldValue (an empty list treated as absent is only safe as long as the
// template renders both the same way).
//
// Every field of every provider CRD is run in four shapes: a real value, an empty one, removed,
// and added to a spec that did not have it.
func TestNoRolloutMeansNoChangeToTheObject(t *testing.T) {
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			contract := loadContract(t, fixture.contractPath)
			fields := crdSpecFields(t, fixture.crdPath)
			required := crdRequiredPaths(t, fixture.crdPath)
			minimal := minimalSpec(t, fixture.instanceClass, required)

			for _, field := range fields {
				for _, base := range []struct {
					name string
					spec map[string]any
				}{
					{name: "from fixture", spec: fixture.instanceClass},
					{name: "from minimal", spec: minimal},
				} {
					for _, value := range []struct {
						name  string
						apply func(spec map[string]any)
					}{
						{name: "set", apply: func(spec map[string]any) { setPath(spec, field.path, crdSampleValue(field.kind)) }},
						{name: "empty", apply: func(spec map[string]any) { setPath(spec, field.path, crdZeroValue(field.kind)) }},
						{name: "removed", apply: func(spec map[string]any) { deletePath(spec, field.path) }},
					} {
						t.Run(field.path+"/"+base.name+"/"+value.name, func(t *testing.T) {
							before := deepCopySpec(t, base.spec)
							after := deepCopySpec(t, before)
							value.apply(after)

							changes, err := Changes(before, after, contract.RolloutFields)
							require.NoError(t, err)
							if len(changes) > 0 {
								return // a rollout was decided; whether the object changes is not this test's question
							}

							beforeObject, beforeErr := renderV2Spec(fixture, contract, before)
							afterObject, afterErr := renderV2Spec(fixture, contract, after)
							if beforeErr != nil || afterErr != nil {
								// An InstanceClass the template refuses is not a shape a user can
								// have — the reconcile fails before anything is created.
								return
							}

							assert.Equal(t, beforeObject, afterObject,
								"changing %s produced a different MachineTemplate, but node-controller decided not to roll: "+
									"the user's change would never reach the machines", field.path)
						})
					}
				}
			}
		})
	}
}

// TestEmptyValueDivergence pins the one place v2 rolls where v1 did not: writing an EMPTY value
// into a field that had none.
//
// The v1 checksums gated such fields on truthiness, so `additionalTags: {}` hashed exactly like no
// additionalTags at all. v2 compares values, and an empty map is not a missing one. Folding the
// two together was tried and reverted: several templates (openstack's imageName/mainNetwork among
// them) gate on hasKey, where an empty value renders a DIFFERENT object — folding would have meant
// the user's edit never reaching the machines, which is the worse of the two failures.
//
// So this is a rollout the user did not strictly need, in exchange for never losing an edit. It
// cannot hit an existing cluster on upgrade: adoption stores the spec as it is.
func TestEmptyValueDivergence(t *testing.T) {
	fixture := fixtureByName(t, "openstack")
	contract := loadContract(t, fixture.contractPath)
	checksumTemplate, err := os.ReadFile(fixture.legacyPath("instance-class.checksum"))
	require.NoError(t, err)

	without := deepCopySpec(t, fixture.instanceClass)
	delete(without, "additionalTags")
	withEmpty := deepCopySpec(t, without)
	withEmpty["additionalTags"] = map[string]any{}

	assert.Equal(t,
		renderLegacyChecksum(t, fixture, checksumTemplate, without, ""),
		renderLegacyChecksum(t, fixture, checksumTemplate, withEmpty, ""),
		"v1 hashed an empty map exactly like a missing field")

	changes, err := Changes(without, withEmpty, contract.RolloutFields)
	require.NoError(t, err)
	assert.Len(t, changes, 1, "v2 sees the empty map as a change")

	before, err := renderV2Spec(fixture, contract, without)
	require.NoError(t, err)
	after, err := renderV2Spec(fixture, contract, withEmpty)
	require.NoError(t, err)
	assert.Equal(t, before, after,
		"and the object is identical, which is what makes this rollout unnecessary but harmless")
}

// TestOpenstackPreemptibleRenderedInV2 pins that OpenStackInstanceClass.spec.preemptible flows into
// the OpenStackMachineTemplate under the v2 contract as the raw Nova tag `preemptible` next to the
// mandatory deckhouse-<uuid>=1 / role-* / use-cluster-api=1. If the raw form regresses (e.g. the
// tag gets rendered as `preemptible=true` key=value), Selectel's preemption mechanism stops firing.
//
// The openstack parity fixture is a Selectel-hosted cluster (connection.authURL points at
// selcloud.ru) — this is what makes the template emit the tag at all.
func TestOpenstackPreemptibleRenderedInV2(t *testing.T) {
	fixture := fixtureByName(t, "openstack")
	contract := loadContract(t, fixture.contractPath)

	spec := deepCopySpec(t, fixture.instanceClass)
	spec["preemptible"] = true

	obj, err := renderV2Spec(fixture, contract, spec)
	require.NoError(t, err)

	tags, ok := obj["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["tags"].([]any)
	require.True(t, ok, "spec.template.spec.tags must be a list")

	assert.Contains(t, tags, "preemptible", "raw Nova tag must be present as a string (no key=value)")
	assert.Contains(t, tags, "deckhouse-"+parityClusterUUID+"=1", "mandatory safety-controller tag")
	assert.Contains(t, tags, "role-deckhouse-"+parityNodeGroup+"-"+parityZone+"=1")
	assert.Contains(t, tags, "use-cluster-api=1")
}

// TestOpenstackPreemptibleNotEmittedOffSelectel pins the other half of the Selectel gate: on any
// non-Selectel OpenStack the template must NOT emit the `preemptible` tag, even when the operator
// set spec.preemptible: true. The tag has no meaning off Selectel and would only look like a
// request Nova silently ignored — the OpenStackPreemptibleUnsupportedProvider alert takes over.
func TestOpenstackPreemptibleNotEmittedOffSelectel(t *testing.T) {
	fixture := fixtureByName(t, "openstack")
	contract := loadContract(t, fixture.contractPath)

	// Deep-copy provider config and swap authURL to something that clearly isn't Selectel.
	fixture.providerConfig = deepCopySpec(t, fixture.providerConfig)
	fixture.providerConfig["connection"] = map[string]any{
		"authURL": "https://public.infra.mail.ru:5000/v3/",
	}

	spec := deepCopySpec(t, fixture.instanceClass)
	spec["preemptible"] = true

	obj, err := renderV2Spec(fixture, contract, spec)
	require.NoError(t, err)

	tags, ok := obj["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["tags"].([]any)
	require.True(t, ok)

	assert.NotContains(t, tags, "preemptible",
		"non-Selectel cluster must NOT receive the preemptible tag: Nova there would attach and ignore it")
	// The mandatory safety-controller tags stay regardless of authURL.
	assert.Contains(t, tags, "deckhouse-"+parityClusterUUID+"=1")
	assert.Contains(t, tags, "use-cluster-api=1")
}

// TestOpenstackConfigDriveMirrorsExternalNetworkDHCP pins that OpenStackMachineTemplate.spec.configDrive
// is set only when the external network does not provide DHCP — the same gate the MCM template used.
// Pre-fix v2 forced configDrive on unconditionally, which silently changed boot behaviour on custom
// images whose cloud-init datasource_list excludes ConfigDrive or whose kernel lacks the vfat driver.
func TestOpenstackConfigDriveMirrorsExternalNetworkDHCP(t *testing.T) {
	fixture := fixtureByName(t, "openstack")
	contract := loadContract(t, fixture.contractPath)

	render := func(dhcp any) map[string]any {
		t.Helper()
		providerConfig := deepCopySpec(t, fixture.providerConfig)
		if dhcp == nil {
			delete(providerConfig, "externalNetworkDHCP")
		} else {
			providerConfig["externalNetworkDHCP"] = dhcp
		}
		local := fixture
		local.providerConfig = providerConfig
		obj, err := renderV2Spec(local, contract, fixture.instanceClass)
		require.NoError(t, err)
		spec, _, err := nestedMap(obj, "spec", "template", "spec")
		require.NoError(t, err)
		return spec
	}

	// externalNetworkDHCP = false -> configDrive must be set, so cloud-init can read metadata
	// from the drive instead of waiting for DHCP that never comes.
	assert.Equal(t, true, render(false)["configDrive"],
		"configDrive must be set when externalNetworkDHCP is false")

	// externalNetworkDHCP = true -> configDrive must be absent, matching the MCM template.
	_, present := render(true)["configDrive"]
	assert.False(t, present,
		"configDrive must be absent when externalNetworkDHCP is true (MCM parity)")

	// externalNetworkDHCP absent -> treated as true, matching the openstack_cluster_configuration
	// hook which defaults the field to true before publishing internal values.
	_, present = render(nil)["configDrive"]
	assert.False(t, present,
		"configDrive must be absent when externalNetworkDHCP is unset — hook default is true")
}

// TestOpenstackPortsRenderedForAllNetworks pins that ports[] is emitted for every network listed
// in mainNetwork + additionalNetworks, regardless of whether any SecurityGroup is configured and
// regardless of podNetworkMode. Pre-fix the gate `if or (SG>0) (podNetworkMode == DRwPSE)` meant
// that a NodeGroup with additionalNetworks but no SG and non-DirectRouting pod mode (e.g. VXLAN)
// emitted no ports[] at all, so CAPO fell back to OpenStackCluster.spec.network — a single
// network — and the secondary NIC silently disappeared on CAPI while MCM happily attached it.
func TestOpenstackPortsRenderedForAllNetworks(t *testing.T) {
	fixture := fixtureByName(t, "openstack")
	contract := loadContract(t, fixture.contractPath)

	// Pathological combo: no SecurityGroups anywhere + VXLAN pod networking.
	providerConfig := deepCopySpec(t, fixture.providerConfig)
	providerConfig["podNetworkMode"] = "VXLAN"
	instances, _, err := nestedMap(providerConfig, "instances")
	require.NoError(t, err)
	delete(instances, "securityGroups")
	providerConfig["instances"] = instances
	local := fixture
	local.providerConfig = providerConfig

	instanceClass := deepCopySpec(t, fixture.instanceClass)
	delete(instanceClass, "additionalSecurityGroups")

	obj, err := renderV2Spec(local, contract, instanceClass)
	require.NoError(t, err)

	ports, found, err := nestedSlice(obj, "spec", "template", "spec", "ports")
	require.NoError(t, err)
	require.True(t, found, "spec.template.spec.ports must be emitted even without SG and off DirectRouting+PortSecurity")
	require.Len(t, ports, 2, "both mainNetwork and additionalNetworks must appear as ports")

	// Neither port carries a securityGroups block (nothing was configured) nor allowedAddressPairs
	// (podNetworkMode is VXLAN, allowedAddressPairs is reserved for DirectRouting+PortSecurity).
	for i, p := range ports {
		port, ok := p.(map[string]any)
		require.True(t, ok, "ports[%d] must be a map", i)
		_, hasSG := port["securityGroups"]
		assert.False(t, hasSG, "ports[%d]: no SG configured, port must not carry securityGroups", i)
		_, hasAAP := port["allowedAddressPairs"]
		assert.False(t, hasAAP, "ports[%d]: podNetworkMode is VXLAN, no allowedAddressPairs expected", i)
	}

	// spec.securityGroups block must also be absent at the machine level.
	_, hasMachineSG, err := nestedSlice(obj, "spec", "template", "spec", "securityGroups")
	require.NoError(t, err)
	assert.False(t, hasMachineSG,
		"machine-level securityGroups must stay absent when no SG is configured in InstanceClass or provider")
}

func TestOpenstackRootVolumeAZOmittedWhenCinderZoneMissing(t *testing.T) {
	fixture := fixtureByName(t, "openstack")
	contract := loadContract(t, fixture.contractPath)

	// Nova AZ (parityZone) is not among Cinder's AZs — the Selectel / zVirt-max case.
	fixture.providerConfig = deepCopySpec(t, fixture.providerConfig)
	fixture.providerConfig["volumeAvailabilityZones"] = []any{"nova"}

	obj, err := renderV2Spec(fixture, contract, fixture.instanceClass)
	require.NoError(t, err)

	rootVolume, ok := obj["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["rootVolume"].(map[string]any)
	require.True(t, ok, "rootVolume must still be emitted — rootDiskSize is set in the fixture")

	_, azPresent := rootVolume["availabilityZone"]
	assert.False(t, azPresent,
		"rootVolume.availabilityZone must be omitted when the Machine's zone is not in Cinder's AZ set; "+
			"keeping it would reproduce the 'Availability zone … is invalid' HTTP 400 from Cinder")

	// And the inverse: when Cinder exposes the Machine's zone, the block is emitted
	// (same path as the main parity fixture; asserted here to catch a regression that would make
	// the two branches indistinguishable).
	fixture.providerConfig["volumeAvailabilityZones"] = []any{parityZone}
	obj, err = renderV2Spec(fixture, contract, fixture.instanceClass)
	require.NoError(t, err)
	rootVolume = obj["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["rootVolume"].(map[string]any)
	az, ok := rootVolume["availabilityZone"].(map[string]any)
	require.True(t, ok, "rootVolume.availabilityZone must be emitted when the Machine's zone is in Cinder's AZ set")
	assert.Equal(t, "Machine", az["from"])
}

// TestOpenstackRootVolumeAZKeptWhenCinderZonesUnknown pins the "do no harm on upgrade" half of
// the gating fix. A missing or empty .provider.volumeAvailabilityZones means the discoverer has
// not published the list yet — not that Cinder knows no zones. On a cluster still running an
// older discoverer image, or on the first reconcile right after the deckhouse controller rolls,
// the registration secret has no volumeAvailabilityZones key, and the template must behave the
// way it did before this change: emit rootVolume.availabilityZone unchanged. Otherwise every
// pre-existing CloudEphemeral node on an unrelated cloud would roll on upgrade to a shape that
// omits the AZ, which is both unnecessary (nothing was broken for them) and churny (fleet-wide
// recreate).
func TestOpenstackRootVolumeAZKeptWhenCinderZonesUnknown(t *testing.T) {
	fixture := fixtureByName(t, "openstack")
	contract := loadContract(t, fixture.contractPath)

	for _, tc := range []struct {
		name  string
		apply func(providerConfig map[string]any)
	}{
		{
			name: "field absent",
			apply: func(p map[string]any) {
				delete(p, "volumeAvailabilityZones")
			},
		},
		{
			name: "field empty",
			apply: func(p map[string]any) {
				p["volumeAvailabilityZones"] = []any{}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := fixture
			f.providerConfig = deepCopySpec(t, fixture.providerConfig)
			tc.apply(f.providerConfig)

			obj, err := renderV2Spec(f, contract, f.instanceClass)
			require.NoError(t, err)

			rootVolume, ok := obj["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["rootVolume"].(map[string]any)
			require.True(t, ok, "rootVolume must be emitted — rootDiskSize is set in the fixture")

			az, ok := rootVolume["availabilityZone"].(map[string]any)
			require.True(t, ok,
				"availabilityZone must be emitted when volumeAvailabilityZones is %s — treating a "+
					"missing list as 'no information' keeps clusters on an older discoverer "+
					"rendering the exact same object as before this fix", tc.name)
			assert.Equal(t, "Machine", az["from"])
		})
	}
}

// TestOpenstackVolumeTypeMapIgnoredWithoutRootDiskSize guards the one combination of fields that
// would be silently wrong: setting volumeTypeMap on an InstanceClass that has no rootDiskSize.
// Without rootDiskSize the root volume is local to the hypervisor (not a BFV Cinder volume), so
// Cinder is not involved and no volume type is ever picked. If the template emitted spec.rootVolume
// just because volumeTypeMap is present, CAPO would create a Cinder volume the user never asked
// for and attach a different disk layout than their flavor describes. Keep the volumeTypeMap
// handling strictly inside the rootDiskSize branch — only there does the field have meaning.
func TestOpenstackVolumeTypeMapIgnoredWithoutRootDiskSize(t *testing.T) {
	fixture := fixtureByName(t, "openstack")
	contract := loadContract(t, fixture.contractPath)

	spec := deepCopySpec(t, fixture.instanceClass)
	delete(spec, "rootDiskSize")
	spec["volumeTypeMap"] = map[string]any{
		parityZone: "fast." + parityZone,
	}

	obj, err := renderV2Spec(fixture, contract, spec)
	require.NoError(t, err)

	_, present := obj["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["rootVolume"]
	assert.False(t, present,
		"spec.rootVolume must not be emitted when rootDiskSize is unset, even if volumeTypeMap is "+
			"present — without BFV there is no Cinder volume to type, and conjuring one would "+
			"silently change the disk layout the user picked through flavor")
}

// TestOpenstackVolumeTypeMapRendersType pins the second half of the CloudEphemeral AZ fix:
// OpenStackInstanceClass.spec.volumeTypeMap, the field-level analogue of the long-standing
// OpenStackClusterConfiguration.nodeGroups[].volumeTypeMap for CloudPermanent. On clouds where
// a Cinder volume_type is bound to a single AZ through extra_specs.volume_backend_name — a
// standard multi-AZ pattern, Selectel's `fast.ru-3a` / `fast.ru-3b` is one well-known case —
// omitting rootVolume.type is not enough: the Cinder default type still picks the wrong AZ.
// The operator must name the type per zone, and the template must forward it.
func TestOpenstackVolumeTypeMapRendersType(t *testing.T) {
	fixture := fixtureByName(t, "openstack")
	contract := loadContract(t, fixture.contractPath)

	// Zone matches a map entry — type and AZ both get emitted.
	spec := deepCopySpec(t, fixture.instanceClass)
	spec["volumeTypeMap"] = map[string]any{
		parityZone:   "fast." + parityZone,
		"other-zone": "fast.other-zone",
	}

	obj, err := renderV2Spec(fixture, contract, spec)
	require.NoError(t, err)

	rootVolume, ok := obj["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["rootVolume"].(map[string]any)
	require.True(t, ok, "rootVolume must be emitted — rootDiskSize is set in the fixture")

	assert.Equal(t, "fast."+parityZone, rootVolume["type"],
		"rootVolume.type must come from volumeTypeMap[.zone] — this is what makes Cinder accept the create "+
			"on multi-AZ clouds where the default type is bound to one AZ")

	az, ok := rootVolume["availabilityZone"].(map[string]any)
	require.True(t, ok, "availabilityZone must be pinned to Machine when the operator chose the type explicitly")
	assert.Equal(t, "Machine", az["from"])

	// Priority: with the map set, the volumeAvailabilityZones gating MUST NOT override. Simulate
	// an environment where the discoverer did not pick parityZone up (e.g. Selectel's Cinder
	// list disagreeing with reality) — the map still wins.
	fixture.providerConfig = deepCopySpec(t, fixture.providerConfig)
	fixture.providerConfig["volumeAvailabilityZones"] = []any{"nova"} // parityZone deliberately absent

	obj, err = renderV2Spec(fixture, contract, spec)
	require.NoError(t, err)
	rootVolume = obj["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["rootVolume"].(map[string]any)
	assert.Equal(t, "fast."+parityZone, rootVolume["type"],
		"volumeTypeMap must override the volumeAvailabilityZones gating — explicit operator intent wins")
	az, ok = rootVolume["availabilityZone"].(map[string]any)
	require.True(t, ok, "availabilityZone must stay pinned when the map covers this zone, regardless of discoverer")
	assert.Equal(t, "Machine", az["from"])

	// Map entry missing for this zone — fall back to the gating path. parityZone is not in
	// volumeAvailabilityZones, so the AZ block must be omitted (TestOpenstackRootVolumeAZOmitted*
	// covers the same branch without the map; here we verify the map's presence does not break it).
	spec["volumeTypeMap"] = map[string]any{"different-zone": "fast.different-zone"}
	obj, err = renderV2Spec(fixture, contract, spec)
	require.NoError(t, err)
	rootVolume = obj["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["rootVolume"].(map[string]any)
	_, typePresent := rootVolume["type"]
	assert.False(t, typePresent,
		"rootVolume.type must not be emitted when volumeTypeMap has no entry for the Machine's zone")
	_, azPresent := rootVolume["availabilityZone"]
	assert.False(t, azPresent,
		"availabilityZone must be omitted when neither the map nor volumeAvailabilityZones cover the zone — "+
			"same fallback that avoids HTTP 400 from Cinder")
}

// TestProviderRenderParityOnEdgeSpecs runs both engines on the InstanceClass shapes a fixture
// never has: only the CRD-required fields, and every optional field set to its zero value.
//
// This is where a mechanical migration is most likely to have drifted. The v1 templates gate
// optional fields on truthiness (`{{- if .nodeGroup.instanceClass.rootDiskSize }}`), while the v2
// ones must use `get`/`hasKey` to survive missingkey=error — and `hasKey` says yes for a field that
// is present and zero, where truthiness says no. A user with `rootDiskSize: 0` or an empty list in
// their InstanceClass would then get a different object out of the two engines.
func TestProviderRenderParityOnEdgeSpecs(t *testing.T) {
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			contract := loadContract(t, fixture.contractPath)
			fields := crdSpecFields(t, fixture.crdPath)
			required := crdRequiredPaths(t, fixture.crdPath)

			specs := map[string]map[string]any{
				"only required fields":     minimalSpec(t, fixture.instanceClass, required),
				"optional fields zeroed":   zeroedSpec(t, fixture.instanceClass, fields, required),
				"optional fields absent":   minimalSpec(t, fixture.instanceClass, required),
				"fixture with empty lists": emptyListSpec(t, fixture.instanceClass),
			}

			for name, spec := range specs {
				t.Run(name, func(t *testing.T) {
					v1Object, v1Err := renderLegacySpec(t, fixture, spec)
					v2Object, v2Err := renderV2Spec(fixture, contract, spec)
					if v2Err == nil {
						v2Object = fixture.withoutPostV1Fields(t, v2Object)
					}

					if v1Err != nil {
						require.Error(t, v2Err,
							"v1 refuses this InstanceClass but v2 renders it — the migration changed what is accepted")
						return
					}
					require.NoError(t, v2Err,
						"v1 renders this InstanceClass but v2 refuses it — the migration changed what is accepted")
					assert.Equal(t, v1Object, v2Object,
						"the two engines disagree on an edge-case InstanceClass")
				})
			}
		})
	}
}

// TestProviderTemplateMatchesRegistrationGVK guards the one place the contract states the same
// fact twice: the apiVersion/kind literal at the top of the provider's template, and
// capiMachineTemplateKind/capiMachineTemplateAPIVersion in the provider's registration secret.
//
// node-controller compares them at generation time and refuses to create an object when they
// disagree — which means a mismatch shipped in a release stops machines from being created at all,
// and nothing before this test would have caught it.
func TestProviderTemplateMatchesRegistrationGVK(t *testing.T) {
	for _, fixture := range providerFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			contract := loadContract(t, fixture.contractPath)
			rendered, err := renderV2Spec(fixture, contract, fixture.instanceClass)
			require.NoError(t, err)

			registration, err := os.ReadFile(fixture.registrationPath)
			require.NoError(t, err)

			assert.Equal(t, registrationValue(t, registration, "capiMachineTemplateKind"), rendered["kind"],
				"the template renders a different kind than the registration secret declares")
			assert.Equal(t, registrationValue(t, registration, "capiMachineTemplateAPIVersion"), rendered["apiVersion"],
				"the template renders a different apiVersion than the registration secret declares")
		})
	}
}

// registrationValue pulls a literal out of `key: {{ b64enc "value" | quote }}`, the shape every
// provider's registration template uses for these two fields.
func registrationValue(t *testing.T, registration []byte, key string) string {
	t.Helper()
	pattern := regexp.MustCompile(key + `:\s*{{\s*b64enc\s+"([^"]+)"`)
	match := pattern.FindSubmatch(registration)
	require.NotNil(t, match, "%s must be declared as a literal in the registration template", key)
	return string(match[1])
}

// crdRequiredPaths collects the dot-paths the CRD marks required, at every level.
func crdRequiredPaths(t *testing.T, path string) map[string]struct{} {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	crd := map[string]any{}
	require.NoError(t, sigsyaml.Unmarshal(data, &crd))

	versions, _, err := nestedSlice(crd, "spec", "versions")
	require.NoError(t, err)

	out := map[string]struct{}{}
	for _, version := range versions {
		versionMap, ok := version.(map[string]any)
		if !ok {
			continue
		}
		spec, found, err := nestedMap(versionMap, "schema", "openAPIV3Schema", "properties", "spec")
		require.NoError(t, err)
		if !found {
			continue
		}
		collectRequired(spec, "", out)
	}
	return out
}

func collectRequired(schema map[string]any, prefix string, out map[string]struct{}) {
	if list, ok := schema["required"].([]any); ok {
		for _, raw := range list {
			name, ok := raw.(string)
			if !ok {
				continue
			}
			path := name
			if prefix != "" {
				path = prefix + "." + name
			}
			out[path] = struct{}{}
		}
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return
	}
	for name, raw := range properties {
		property, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		collectRequired(property, path, out)
	}
}

// minimalSpec keeps only the values the CRD requires (a nested map survives when anything under it
// does), which is the smallest InstanceClass a user can actually create.
func minimalSpec(t *testing.T, spec map[string]any, required map[string]struct{}) map[string]any {
	t.Helper()
	out := map[string]any{}
	for key, value := range deepCopySpec(t, spec) {
		keep(out, key, key, value, required)
	}
	return out
}

func keep(out map[string]any, path, key string, value any, required map[string]struct{}) {
	nested, isMap := value.(map[string]any)
	if !isMap {
		if _, ok := required[path]; ok {
			out[key] = value
		}
		return
	}

	kept := map[string]any{}
	for nestedKey, nestedValue := range nested {
		keep(kept, path+"."+nestedKey, nestedKey, nestedValue, required)
	}
	if len(kept) > 0 {
		out[key] = kept
	}
}

// zeroedSpec sets every optional CRD field to the zero value of its declared type — the shape a
// user creates by writing `rootDiskSize: 0` or `additionalTags: {}`.
func zeroedSpec(t *testing.T, spec map[string]any, fields []crdField, required map[string]struct{}) map[string]any {
	t.Helper()
	out := deepCopySpec(t, spec)
	for _, field := range fields {
		if _, isRequired := required[field.path]; isRequired {
			continue
		}
		if _, present := fieldPresent(out, field.path); !present {
			continue
		}
		setPath(out, field.path, crdZeroValue(field.kind))
	}
	return out
}

// emptyListSpec replaces every list in the fixture with an empty one: v1 skipped an empty list by
// truthiness, and a v2 template that switched to hasKey would render an empty block instead.
func emptyListSpec(t *testing.T, spec map[string]any) map[string]any {
	t.Helper()
	out := deepCopySpec(t, spec)
	emptyLists(out)
	return out
}

func emptyLists(spec map[string]any) {
	for key, value := range spec {
		switch typed := value.(type) {
		case []any:
			spec[key] = []any{}
		case map[string]any:
			emptyLists(typed)
		}
	}
}

func fieldPresent(spec map[string]any, path string) (any, bool) {
	current := any(spec)
	for _, segment := range strings.Split(path, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func crdZeroValue(kind string) any {
	switch kind {
	case "integer", "number":
		return float64(0)
	case "boolean":
		return false
	case "array":
		return []any{}
	case "object":
		return map[string]any{}
	default:
		return ""
	}
}

// crdSpecFields reads every leaf of .spec out of the provider's InstanceClass CRD. Leaves are
// scalars, arrays and free-form objects; nested objects are walked into, because that is how the
// rolloutFields dot-paths address them (DVP's "virtualMachine.cpu.cores").
func crdSpecFields(t *testing.T, path string) []crdField {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err, "provider InstanceClass CRD must exist")

	crd := map[string]any{}
	require.NoError(t, sigsyaml.Unmarshal(data, &crd))

	versions, _, err := nestedSlice(crd, "spec", "versions")
	require.NoError(t, err)
	require.NotEmpty(t, versions)

	fields := map[string]string{}
	for _, version := range versions {
		versionMap, ok := version.(map[string]any)
		if !ok {
			continue
		}
		spec, found, err := nestedMap(versionMap, "schema", "openAPIV3Schema", "properties", "spec")
		require.NoError(t, err)
		if !found {
			continue
		}
		collectCRDFields(spec, "", fields)
	}

	out := make([]crdField, 0, len(fields))
	for path, kind := range fields {
		out = append(out, crdField{path: path, kind: kind})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

func collectCRDFields(schema map[string]any, prefix string, out map[string]string) {
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return
	}
	for name, raw := range properties {
		property, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		kind, _ := property["type"].(string)
		if kind == "object" {
			if _, hasProperties := property["properties"]; hasProperties {
				collectCRDFields(property, path, out)
				continue
			}
		}
		out[path] = kind
	}
}

// crdSampleValue returns a value of the declared type that differs from anything the fixtures use,
// so setting it is always a real change.
func crdSampleValue(kind string) any {
	switch kind {
	case "integer", "number":
		return float64(4242)
	case "boolean":
		return true
	case "array":
		return []any{"crd-parity"}
	case "object":
		return map[string]any{"crd-parity": "value"}
	default:
		return "crd-parity"
	}
}

func deletePath(spec map[string]any, path string) {
	segments := strings.Split(path, ".")
	current := spec
	for _, segment := range segments[:len(segments)-1] {
		next, ok := current[segment].(map[string]any)
		if !ok {
			return
		}
		current = next
	}
	delete(current, segments[len(segments)-1])
}

func setPath(spec map[string]any, path string, value any) {
	segments := strings.Split(path, ".")
	current := spec
	for _, segment := range segments[:len(segments)-1] {
		next, ok := current[segment].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[segment] = next
		}
		current = next
	}
	current[segments[len(segments)-1]] = value
}

func nestedMap(obj map[string]any, keys ...string) (map[string]any, bool, error) {
	value, found, err := nestedValue(obj, keys...)
	if err != nil || !found {
		return nil, found, err
	}
	out, ok := value.(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("%s is %T, not a map", strings.Join(keys, "."), value)
	}
	return out, true, nil
}

func nestedSlice(obj map[string]any, keys ...string) ([]any, bool, error) {
	value, found, err := nestedValue(obj, keys...)
	if err != nil || !found {
		return nil, found, err
	}
	out, ok := value.([]any)
	if !ok {
		return nil, false, fmt.Errorf("%s is %T, not a list", strings.Join(keys, "."), value)
	}
	return out, true, nil
}

func nestedValue(obj map[string]any, keys ...string) (any, bool, error) {
	current := any(obj)
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("%s is not a map", key)
		}
		current, ok = m[key]
		if !ok {
			return nil, false, nil
		}
	}
	return current, true, nil
}
