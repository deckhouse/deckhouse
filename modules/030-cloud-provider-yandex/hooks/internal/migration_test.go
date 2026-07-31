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

package internal

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	yciccv1 "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/api/instanceclass/v1"
	ycpccv1 "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/api/pcc/v1"
	ycsettingsv1 "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/api/settings/v1"
	ycsettingsv2 "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/api/settings/v2"
	ycmeta "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/meta"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	"github.com/deckhouse/module-sdk/pkg"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
)

var _ = Describe("mapPCCInstanceClassToYandexInstanceClassSpec", func() {
	It("all fields populated", func() {
		ic := ycpccv1.YandexInstanceClass{
			Cores:            4,
			CoreFraction:     ptr.To(50),
			Memory:           8192,
			ImageID:          "fd85m9q2qspfnsv055rh",
			Platform:         ptr.To("standard-v3"),
			DiskSizeGB:       ptr.To(50),
			DiskType:         ptr.To("network-ssd"),
			NetworkType:      ptr.To("SoftwareAccelerated"),
			AdditionalLabels: map[string]string{"env": "prod"},
			ExternalSubnetID: ptr.To("subnet-abc"),
			ExternalSubnetIDs: []string{
				"subnet-extra-1",
				"subnet-extra-2",
			},
		}

		spec := mapPCCInstanceClassToYandexInstanceClassSpec(ic)

		Expect(spec.Cores).To(Equal(4))
		Expect(spec.CoreFraction).To(Equal(50))
		Expect(spec.Memory).To(Equal(8192))
		Expect(spec.ImageID).To(Equal("fd85m9q2qspfnsv055rh"))
		Expect(spec.PlatformID).To(Equal("standard-v3"))
		Expect(spec.DiskSizeGB).To(Equal(50))
		Expect(spec.DiskType).To(Equal("network-ssd"))
		Expect(spec.NetworkType).To(Equal("SoftwareAccelerated"))
		Expect(spec.AdditionalLabels).To(Equal(map[string]string{"env": "prod"}))
		Expect(spec.MainSubnet).To(Equal("subnet-abc"))
		Expect(spec.AdditionalSubnets).To(Equal([]string{"subnet-extra-1", "subnet-extra-2"}))
	})

	It("nil optional fields", func() {
		ic := ycpccv1.YandexInstanceClass{
			Cores:   2,
			Memory:  4096,
			ImageID: "fd-image",
		}

		spec := mapPCCInstanceClassToYandexInstanceClassSpec(ic)

		Expect(spec.Cores).To(Equal(2))
		Expect(spec.Memory).To(Equal(4096))
		Expect(spec.ImageID).To(Equal("fd-image"))
		Expect(spec.CoreFraction).To(Equal(0))
		Expect(spec.PlatformID).To(Equal(""))
		Expect(spec.DiskSizeGB).To(Equal(0))
		Expect(spec.DiskType).To(Equal(""))
		Expect(spec.NetworkType).To(Equal(""))
		Expect(spec.AdditionalLabels).To(BeNil())
		Expect(spec.MainSubnet).To(Equal(""))
		Expect(spec.AdditionalSubnets).To(BeNil())
	})

	It("with AdditionalLabels", func() {
		ic := ycpccv1.YandexInstanceClass{
			Cores:            4,
			Memory:           8192,
			ImageID:          "fd-image",
			AdditionalLabels: map[string]string{"project": "cms", "severity": "critical"},
		}

		spec := mapPCCInstanceClassToYandexInstanceClassSpec(ic)

		Expect(spec.AdditionalLabels).To(Equal(map[string]string{
			"project":  "cms",
			"severity": "critical",
		}))
	})

	It("with ExternalSubnetIDs", func() {
		ic := ycpccv1.YandexInstanceClass{
			Cores:             2,
			Memory:            4096,
			ImageID:           "fd-image",
			ExternalSubnetIDs: []string{"s1", "s2"},
		}

		spec := mapPCCInstanceClassToYandexInstanceClassSpec(ic)

		Expect(spec.AdditionalSubnets).To(Equal([]string{"s1", "s2"}))
	})
})

var _ = Describe("resolveZones", func() {
	It("uses node-group zones", func() {
		result := resolveZones(
			[]string{"ru-central1-a", "ru-central1-b"},
			[]string{"ru-central1-d"},
		)
		Expect(result).To(Equal([]interface{}{"ru-central1-a", "ru-central1-b"}))
	})

	It("falls back to cluster zones", func() {
		result := resolveZones(
			nil,
			[]string{"ru-central1-a", "ru-central1-b"},
		)
		Expect(result).To(Equal([]interface{}{"ru-central1-a", "ru-central1-b"}))
	})

	It("both empty returns nil", func() {
		result := resolveZones(nil, nil)
		Expect(result).To(BeNil())
	})

	It("empty strings filtered from ng zones", func() {
		result := resolveZones(
			[]string{"ru-central1-a", "", "ru-central1-b"},
			nil,
		)
		Expect(result).To(Equal([]interface{}{"ru-central1-a", "ru-central1-b"}))
	})

	It("empty strings filtered from cluster zones with ng zones having only empty strings", func() {
		result := resolveZones(
			[]string{""},
			[]string{"ru-central1-a", ""},
		)
		Expect(result).To(Equal([]interface{}{"ru-central1-a"}))
	})

	It("ng zones with only empty strings falls back to cluster", func() {
		result := resolveZones(
			[]string{"", ""},
			[]string{"ru-central1-a"},
		)
		Expect(result).To(Equal([]interface{}{"ru-central1-a"}))
	})
})

var _ = Describe("BuildNodeGroupAndInstanceClassResources", func() {
	It("master node: creates NodeGroup with master labels and InstanceClass", func() {
		nodeTemplate := map[string]any{
			"labels": map[string]any{
				"node-role.kubernetes.io/control-plane": "",
				"node-role.kubernetes.io/master":        "",
			},
		}

		ngResource, icResource, err := BuildNodeGroupAndInstanceClassResources(
			"master",
			3,
			[]string{"ru-central1-a", "ru-central1-b", "ru-central1-d"},
			ycpccv1.YandexInstanceClass{
				Cores:   4,
				Memory:  8192,
				ImageID: "fd-master-image",
			},
			nodeTemplate,
			[]string{},
		)
		Expect(err).NotTo(HaveOccurred())

		// InstanceClass CRD metadata
		Expect(icResource.APIVersion).To(Equal(yciccv1.SchemeGroupVersion.String()))
		Expect(icResource.Kind).To(Equal(yciccv1.YandexInstanceClassKind))
		Expect(icResource.Name).To(Equal(cpapi.BuildInstanceClassName("master")))
		Expect(icResource.Spec.Cores).To(Equal(4))
		Expect(icResource.Spec.Memory).To(Equal(8192))
		Expect(icResource.Spec.ImageID).To(Equal("fd-master-image"))

		// NodeGroup spec
		Expect(ngResource["apiVersion"]).To(Equal("deckhouse.io/v1"))
		Expect(ngResource["kind"]).To(Equal("NodeGroup"))
		spec, ok := ngResource["spec"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(spec["nodeType"]).To(Equal("CloudPermanent"))

		// Cloud instances
		cloudInstances, ok := spec["cloudInstances"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(cloudInstances["minPerZone"]).To(Equal(int64(3)))
		Expect(cloudInstances["maxPerZone"]).To(Equal(int64(3)))

		classRef, ok := cloudInstances["classReference"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(classRef["kind"]).To(Equal(yciccv1.YandexInstanceClassKind))

		// Zones
		zones, ok := cloudInstances["zones"].([]any)
		Expect(ok).To(BeTrue())
		Expect(zones).To(HaveLen(3))

		// Master labels in nodeTemplate
		nodeTmpl, ok := spec["nodeTemplate"].(map[string]any)
		Expect(ok).To(BeTrue())
		labels, ok := nodeTmpl["labels"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(labels).To(HaveKey("node-role.kubernetes.io/control-plane"))
		Expect(labels).To(HaveKey("node-role.kubernetes.io/master"))
	})

	It("regular node: NodeGroup with CloudPermanent, replicas, cluster zone fallback", func() {
		ngResource, icResource, err := BuildNodeGroupAndInstanceClassResources(
			"worker",
			2,
			nil,
			ycpccv1.YandexInstanceClass{
				Cores:    8,
				Memory:   16384,
				ImageID:  "fd-worker-image",
				Platform: ptr.To("standard-v3"),
			},
			nil,
			[]string{"ru-central1-a", "ru-central1-b"},
		)
		Expect(err).NotTo(HaveOccurred())

		// InstanceClass
		Expect(icResource.Name).To(ContainSubstring("worker"))
		Expect(icResource.Spec.Cores).To(Equal(8))
		Expect(icResource.Spec.Memory).To(Equal(16384))
		Expect(icResource.Spec.ImageID).To(Equal("fd-worker-image"))
		Expect(icResource.Spec.PlatformID).To(Equal("standard-v3"))

		// NodeGroup
		spec, ok := ngResource["spec"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(spec["nodeType"]).To(Equal("CloudPermanent"))
		Expect(spec["nodeTemplate"]).To(BeNil())

		cloudInstances, ok := spec["cloudInstances"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(cloudInstances["minPerZone"]).To(Equal(int64(2)))
		Expect(cloudInstances["maxPerZone"]).To(Equal(int64(2)))

		zones, ok := cloudInstances["zones"].([]any)
		Expect(ok).To(BeTrue())
		Expect(zones).To(ConsistOf("ru-central1-a", "ru-central1-b"))
	})
})

var _ = Describe("BuildModuleConfigSettingsV2", func() {
	It("builds with provider, nodes, storage, ccm settings", func() {
		pcc := ycpccv1.YandexProviderClusterConfiguration{
			Layout:         "Standard",
			SSHPublicKey:   "ssh-rsa AAAAB3...",
			NodeNetworkCIDR: "10.0.0.0/16",
			Labels: map[string]string{
				"environment": "production",
			},
			Zones: []string{"ru-central1-a", "ru-central1-b"},
			Provider: ycpccv1.YandexProvider{
				CloudID:            "cloud-123",
				FolderID:           "folder-456",
				ServiceAccountJSON: `{"id":"sa"}`,
			},
		}

		mc := ycsettingsv1.ModuleConfigSettings{
			StorageClass: ycsettingsv1.StorageClassSettings{
				Exclude: []string{"network-hdd"},
			},
			AdditionalExternalNetworkIDs: []string{"net-extra"},
		}

		result := BuildModuleConfigSettingsV2(pcc, mc)

		Expect(result.Provider.Parameters.CloudID).To(Equal("cloud-123"))
		Expect(result.Provider.Parameters.FolderID).To(Equal("folder-456"))
		Expect(result.Nodes.Parameters.SSHPublicKey).To(Equal("ssh-rsa AAAAB3..."))
		Expect(result.Nodes.Parameters.Layout).To(Equal("Standard"))
		Expect(result.Nodes.Parameters.NodeNetworkCIDR).To(Equal("10.0.0.0/16"))
		Expect(result.Nodes.Parameters.Labels).To(Equal(map[string]string{"environment": "production"}))
		Expect(result.Nodes.Parameters.Zones).To(Equal([]string{"ru-central1-a", "ru-central1-b"}))
		Expect(result.Nodes.Disabled).To(BeFalse())
		Expect(result.Storage.Parameters.ExcludedStorageClasses).To(Equal([]string{"network-hdd"}))
		Expect(result.Storage.Disabled).To(BeFalse())
		Expect(result.CCM.Parameters.AdditionalExternalNetworkIDs).To(Equal([]string{"net-extra"}))
		Expect(result.CCM.Disabled).To(BeFalse())
	})

	It("nil optional fields stay empty", func() {
		pcc := ycpccv1.YandexProviderClusterConfiguration{
			Layout:       "WithoutNAT",
			SSHPublicKey: "ssh-rsa KEY",
			NodeNetworkCIDR: "10.0.0.0/16",
			Provider: ycpccv1.YandexProvider{
				CloudID:  "cloud-1",
				FolderID: "folder-1",
			},
		}
		mc := ycsettingsv1.ModuleConfigSettings{}

		result := BuildModuleConfigSettingsV2(pcc, mc)

		Expect(result.Nodes.Parameters.WithNATInstance).To(Equal(ycsettingsv2.NATInstanceParameters{}))
		Expect(result.Nodes.Parameters.DHCPOptions).To(Equal(ycsettingsv2.DHCPOptions{}))
		Expect(result.Nodes.Parameters.ExistingNetworkID).To(Equal(""))
		Expect(result.Nodes.Parameters.ExistingZoneToSubnetIDMap).To(BeNil())
		Expect(result.Nodes.Parameters.Labels).To(BeNil())
		Expect(result.Nodes.Parameters.Zones).To(BeNil())
		Expect(result.Storage.Parameters.ExcludedStorageClasses).To(BeNil())
		Expect(result.CCM.Parameters.AdditionalExternalNetworkIDs).To(BeNil())
	})
})

var _ = Describe("BuildCredentialsSecrets", func() {
	It("creates d8-credentials secret when serviceAccountJSON is present", func() {
		pcc := ycpccv1.YandexProviderClusterConfiguration{
			Provider: ycpccv1.YandexProvider{
				ServiceAccountJSON: `{"id":"my-sa"}`,
			},
		}

		secrets := BuildCredentialsSecrets(pcc)
		Expect(secrets).To(HaveLen(1))
		Expect(secrets[0].Name).To(Equal(cpapi.CredentialSecretName))
		Expect(secrets[0].Namespace).To(Equal(ycmeta.Namespace))
		Expect(string(secrets[0].Type)).To(Equal(cpapi.CredentialsSecretType))
		Expect(secrets[0].StringData[cpapi.CredentialSecretAuthSchemeKey]).To(Equal(string(cpapi.AuthSchemeServiceAccount)))
		Expect(secrets[0].StringData[cpapi.CredentialSecretSecretKey]).To(Equal(`{"id":"my-sa"}`))
	})

	It("creates exporter credential secret when ExporterAPIKey is present", func() {
		pcc := ycpccv1.YandexProviderClusterConfiguration{
			WithNATInstance: &ycpccv1.YandexWithNATInstance{
				ExporterAPIKey: ptr.To("exporter-key-123"),
			},
		}

		secrets := BuildCredentialsSecrets(pcc)
		Expect(secrets).To(HaveLen(1))
		Expect(secrets[0].Name).To(Equal(ycmeta.ExporterCredentialSecretName))
		Expect(string(secrets[0].Type)).To(Equal(cpapi.CredentialsSecretType))
		Expect(secrets[0].StringData[cpapi.CredentialSecretAuthSchemeKey]).To(Equal(string(cpapi.AuthSchemeAPIToken)))
		Expect(secrets[0].StringData[cpapi.CredentialSecretSecretKey]).To(Equal("exporter-key-123"))
	})

	It("creates both secrets when both credentials are present", func() {
		pcc := ycpccv1.YandexProviderClusterConfiguration{
			Provider: ycpccv1.YandexProvider{
				ServiceAccountJSON: `{"id":"sa"}`,
			},
			WithNATInstance: &ycpccv1.YandexWithNATInstance{
				ExporterAPIKey: ptr.To("exporter-key"),
			},
		}

		secrets := BuildCredentialsSecrets(pcc)
		Expect(secrets).To(HaveLen(2))
		Expect(secrets[0].Name).To(Equal(cpapi.CredentialSecretName))
		Expect(secrets[1].Name).To(Equal(ycmeta.ExporterCredentialSecretName))
	})

	It("returns empty when no credentials are present", func() {
		pcc := ycpccv1.YandexProviderClusterConfiguration{}
		secrets := BuildCredentialsSecrets(pcc)
		Expect(secrets).To(BeEmpty())
	})
})

// --------------------------------------------------------------------------
// IsMigrationResourcesApplied
// --------------------------------------------------------------------------

type mockSnapshot struct {
	data []byte
}

func (m mockSnapshot) UnmarshalTo(v any) error {
	return json.Unmarshal(m.data, v)
}

func (m mockSnapshot) String() string {
	return string(m.data)
}

type mockSnapshots map[string][]pkg.Snapshot

func (m mockSnapshots) Get(key string) []pkg.Snapshot {
	return m[key]
}

func mustMarshal(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

func snap(m mockSnapshots) pkg.Snapshots {
	return m
}

var _ = Describe("IsMigrationResourcesApplied", func() {
	It("returns false when no ModuleConfig snapshot", func() {
		input := &go_hook.HookInput{Snapshots: snap(mockSnapshots{})}
		Expect(IsMigrationResourcesApplied(input, ycpccv1.YandexProviderClusterConfiguration{})).To(BeFalse())
	})

	It("returns false when ModuleConfig version < 2", func() {
		mc := ModuleConfigFilterResult{Version: 1, Enabled: true, SettingsV2: []byte(`{}`)}
		input := &go_hook.HookInput{
			Snapshots: snap(mockSnapshots{
				"module_config": {mockSnapshot{mustMarshal(mc)}},
			}),
		}
		Expect(IsMigrationResourcesApplied(input, ycpccv1.YandexProviderClusterConfiguration{})).To(BeFalse())
	})

	It("returns false when ModuleConfig is disabled", func() {
		mc := ModuleConfigFilterResult{Version: 2, Enabled: false, SettingsV2: []byte(`{}`)}
		input := &go_hook.HookInput{
			Snapshots: snap(mockSnapshots{
				"module_config": {mockSnapshot{mustMarshal(mc)}},
			}),
		}
		Expect(IsMigrationResourcesApplied(input, ycpccv1.YandexProviderClusterConfiguration{})).To(BeFalse())
	})

	It("returns false when SettingsV2 is empty", func() {
		mc := ModuleConfigFilterResult{Version: 2, Enabled: true}
		input := &go_hook.HookInput{
			Snapshots: snap(mockSnapshots{
				"module_config": {mockSnapshot{mustMarshal(mc)}},
			}),
		}
		Expect(IsMigrationResourcesApplied(input, ycpccv1.YandexProviderClusterConfiguration{})).To(BeFalse())
	})

	It("returns false when no credential secrets", func() {
		mc := ModuleConfigFilterResult{Version: 2, Enabled: true, SettingsV2: []byte(`{}`)}
		input := &go_hook.HookInput{
			Snapshots: snap(mockSnapshots{
				"module_config":       {mockSnapshot{mustMarshal(mc)}},
				"credential_secrets":  {},
			}),
		}
		Expect(IsMigrationResourcesApplied(input, ycpccv1.YandexProviderClusterConfiguration{})).To(BeFalse())
	})

	It("returns true when all resources present (no NodeGroups in PCC)", func() {
		mc := ModuleConfigFilterResult{Version: 2, Enabled: true, SettingsV2: []byte(`{}`)}
		creds := []NamedResourceFilterResult{{Name: "d8-credentials"}}
		ngs := []NamedResourceFilterResult{{Name: "master"}}
		ics := []NamedResourceFilterResult{{Name: cpapi.BuildInstanceClassName("master")}}

		snaps := mockSnapshots{}
		snaps["module_config"] = []pkg.Snapshot{mockSnapshot{mustMarshal(mc)}}
		snaps["credential_secrets"] = snapshotsFrom(creds)
		snaps["node_groups"] = snapshotsFrom(ngs)
		snaps["yandex_instance_classes"] = snapshotsFrom(ics)

		input := &go_hook.HookInput{Snapshots: snap(snaps)}
		pcc := ycpccv1.YandexProviderClusterConfiguration{} // no NodeGroups, no MasterNodeGroup replicas=0
		Expect(IsMigrationResourcesApplied(input, pcc)).To(BeTrue())
	})

	It("returns false when master NodeGroup missing but PCC has masterNodeGroup", func() {
		mc := ModuleConfigFilterResult{Version: 2, Enabled: true, SettingsV2: []byte(`{}`)}
		creds := []NamedResourceFilterResult{{Name: "d8-credentials"}}
		// no "master" node group

		snaps := mockSnapshots{}
		snaps["module_config"] = []pkg.Snapshot{mockSnapshot{mustMarshal(mc)}}
		snaps["credential_secrets"] = snapshotsFrom(creds)
		snaps["node_groups"] = snapshotsFrom([]NamedResourceFilterResult{})
		snaps["yandex_instance_classes"] = snapshotsFrom([]NamedResourceFilterResult{})

		pcc := ycpccv1.YandexProviderClusterConfiguration{
			MasterNodeGroup: ycpccv1.YandexMasterNodeGroup{Replicas: 1},
		}
		input := &go_hook.HookInput{Snapshots: snap(snaps)}
		Expect(IsMigrationResourcesApplied(input, pcc)).To(BeFalse())
	})

	It("returns false when a NodeGroup from PCC is missing", func() {
		mc := ModuleConfigFilterResult{Version: 2, Enabled: true, SettingsV2: []byte(`{}`)}
		creds := []NamedResourceFilterResult{{Name: "d8-credentials"}}
		ngs := []NamedResourceFilterResult{{Name: "master"}}
		ics := []NamedResourceFilterResult{{Name: cpapi.BuildInstanceClassName("master")}}

		snaps := mockSnapshots{}
		snaps["module_config"] = []pkg.Snapshot{mockSnapshot{mustMarshal(mc)}}
		snaps["credential_secrets"] = snapshotsFrom(creds)
		snaps["node_groups"] = snapshotsFrom(ngs)
		snaps["yandex_instance_classes"] = snapshotsFrom(ics)

		pcc := ycpccv1.YandexProviderClusterConfiguration{
			MasterNodeGroup: ycpccv1.YandexMasterNodeGroup{Replicas: 1},
			NodeGroups: []ycpccv1.YandexStaticNodeGroup{
				{Name: "worker"},
			},
		}
		input := &go_hook.HookInput{Snapshots: snap(snaps)}
		Expect(IsMigrationResourcesApplied(input, pcc)).To(BeFalse())
	})

	It("returns true with master and all NodeGroups present", func() {
		mc := ModuleConfigFilterResult{Version: 2, Enabled: true, SettingsV2: []byte(`{}`)}
		creds := []NamedResourceFilterResult{{Name: "d8-credentials"}}
		ngs := []NamedResourceFilterResult{
			{Name: "master"},
			{Name: "worker"},
		}
		ics := []NamedResourceFilterResult{
			{Name: cpapi.BuildInstanceClassName("master")},
			{Name: cpapi.BuildInstanceClassName("worker")},
		}

		snaps := mockSnapshots{}
		snaps["module_config"] = []pkg.Snapshot{mockSnapshot{mustMarshal(mc)}}
		snaps["credential_secrets"] = snapshotsFrom(creds)
		snaps["node_groups"] = snapshotsFrom(ngs)
		snaps["yandex_instance_classes"] = snapshotsFrom(ics)

		pcc := ycpccv1.YandexProviderClusterConfiguration{
			MasterNodeGroup: ycpccv1.YandexMasterNodeGroup{Replicas: 1},
			NodeGroups: []ycpccv1.YandexStaticNodeGroup{
				{Name: "worker"},
			},
		}
		input := &go_hook.HookInput{Snapshots: snap(snaps)}
		Expect(IsMigrationResourcesApplied(input, pcc)).To(BeTrue())
	})

	It("returns false when InstanceClass is missing (only NodeGroup present)", func() {
		mc := ModuleConfigFilterResult{Version: 2, Enabled: true, SettingsV2: []byte(`{}`)}
		creds := []NamedResourceFilterResult{{Name: "d8-credentials"}}
		ngs := []NamedResourceFilterResult{{Name: "master"}}
		// no instance classes

		snaps := mockSnapshots{}
		snaps["module_config"] = []pkg.Snapshot{mockSnapshot{mustMarshal(mc)}}
		snaps["credential_secrets"] = snapshotsFrom(creds)
		snaps["node_groups"] = snapshotsFrom(ngs)
		snaps["yandex_instance_classes"] = snapshotsFrom([]NamedResourceFilterResult{})

		pcc := ycpccv1.YandexProviderClusterConfiguration{
			MasterNodeGroup: ycpccv1.YandexMasterNodeGroup{Replicas: 1},
		}
		input := &go_hook.HookInput{Snapshots: snap(snaps)}
		Expect(IsMigrationResourcesApplied(input, pcc)).To(BeFalse())
	})
})

func snapshotsFrom[T any](items []T) []pkg.Snapshot {
	snaps := make([]pkg.Snapshot, len(items))
	for i, item := range items {
		snaps[i] = mockSnapshot{mustMarshal(item)}
	}
	return snaps
}
