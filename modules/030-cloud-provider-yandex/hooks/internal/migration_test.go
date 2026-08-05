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
	"bytes"
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

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

	// Leaving platformID, diskType and diskSizeGB empty would let the apiserver
	// apply the YandexInstanceClass CRD defaults (standard-v3 / network-hdd),
	// which differ from what terraform used before the migration and would
	// replace the boot and etcd disks of existing clusters.
	It("nil optional fields fall back to the pre-migration terraform defaults", func() {
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
		Expect(spec.PlatformID).To(Equal("standard-v2"))
		Expect(spec.DiskSizeGB).To(Equal(50))
		Expect(spec.DiskType).To(Equal("network-ssd"))
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

	// A node group that declares zones keeps them even when every entry is blank: the
	// cluster-wide zones must not narrow it. Pre-migration terraform behaved the same way —
	// candi/terraform-modules/master-node/main.tf intersects with the node group zones whenever
	// they are set, so an all-blank list ends up selecting every subnet, not the cluster zones.
	It("does not fall back to cluster zones when the node group declares blank zones", func() {
		result := resolveZones(
			[]string{""},
			[]string{"ru-central1-a", ""},
		)
		Expect(result).To(BeNil())
	})

	It("omits zones when both the node group and the cluster declare only blank zones", func() {
		result := resolveZones(
			[]string{"", ""},
			[]string{""},
		)
		Expect(result).To(BeNil())
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
		Expect(icResource.APIVersion).To(Equal(yciccv1.GroupVersionKind.GroupVersion().String()))
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
			Layout:          "Standard",
			SSHPublicKey:    "ssh-rsa AAAAB3...",
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
			Layout:          "WithoutNAT",
			SSHPublicKey:    "ssh-rsa KEY",
			NodeNetworkCIDR: "10.0.0.0/16",
			Provider: ycpccv1.YandexProvider{
				CloudID:  "cloud-1",
				FolderID: "folder-1",
			},
		}
		mc := ycsettingsv1.ModuleConfigSettings{}

		result := BuildModuleConfigSettingsV2(pcc, mc)

		// Optional sections are value types: an absent PCC section stays a zero value.
		// It still serializes as an empty object, which matches the terraform projection —
		// candi/terraform-modules/migration/locals.tf always emits withNATInstance and
		// dhcpOptions with zero values instead of omitting them. Scalars keep omitempty,
		// so nothing carries a bogus value, and natInstanceResources defaults (2/2048/
		// standard-v2) come from the CRD schema on both paths.
		Expect(result.Nodes.Parameters.WithNATInstance).To(Equal(ycsettingsv2.NATInstanceParameters{}))
		Expect(result.Nodes.Parameters.DHCPOptions).To(Equal(ycsettingsv2.DHCPOptions{}))
		Expect(result.Nodes.Parameters.ExistingNetworkID).To(BeEmpty())
		Expect(result.Nodes.Parameters.ExistingZoneToSubnetIDMap).To(BeNil())
		Expect(result.Nodes.Parameters.Labels).To(BeNil())
		Expect(result.Nodes.Parameters.Zones).To(BeNil())
		Expect(result.Storage.Parameters.ExcludedStorageClasses).To(BeNil())
		Expect(result.CCM.Parameters.AdditionalExternalNetworkIDs).To(BeNil())
		Expect(result.Nodes.Parameters.ExternalIPAddresses).To(BeEmpty())
		Expect(result.Nodes.Parameters.ExternalSubnetIDs).To(BeEmpty())

		raw, err := json.Marshal(result)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(raw)).NotTo(ContainSubstring("existingNetworkID"))
		Expect(string(raw)).NotTo(ContainSubstring("internalSubnetCIDR"))
		Expect(string(raw)).NotTo(ContainSubstring("domainName"))
		Expect(string(raw)).To(ContainSubstring(`"withNATInstance":{"natInstanceResources":{}}`))
		Expect(string(raw)).To(ContainSubstring(`"dhcpOptions":{}`))
	})

	// YandexInstanceClass has no externalIPAddresses counterpart, so the
	// per-node-group lists have to survive in nodes.parameters or the reserved
	// addresses of existing clusters are lost on migration.
	It("moves per-node-group external addressing into nodes.parameters", func() {
		pcc := ycpccv1.YandexProviderClusterConfiguration{
			Layout:          "Standard",
			SSHPublicKey:    "ssh-rsa KEY",
			NodeNetworkCIDR: "10.0.0.0/16",
			Provider: ycpccv1.YandexProvider{
				CloudID:  "cloud-1",
				FolderID: "folder-1",
			},
			MasterNodeGroup: ycpccv1.YandexMasterNodeGroup{
				Replicas: 3,
				InstanceClass: ycpccv1.YandexMasterInstanceClass{
					YandexInstanceClass: ycpccv1.YandexInstanceClass{
						Cores:               4,
						Memory:              8192,
						ImageID:             "fd-image",
						ExternalIPAddresses: []string{"203.0.113.1", "203.0.113.2", "Auto"},
						ExternalSubnetIDs:   []string{"subnet-ext-a", "subnet-ext-b", "subnet-ext-d"},
					},
				},
			},
			NodeGroups: []ycpccv1.YandexStaticNodeGroup{
				{
					Name:     "worker",
					Replicas: 1,
					InstanceClass: ycpccv1.YandexStaticInstanceClass{
						YandexInstanceClass: ycpccv1.YandexInstanceClass{
							Cores:               2,
							Memory:              2048,
							ImageID:             "fd-image",
							ExternalIPAddresses: []string{"Auto"},
						},
					},
				},
				{
					Name:     "system",
					Replicas: 1,
					InstanceClass: ycpccv1.YandexStaticInstanceClass{
						YandexInstanceClass: ycpccv1.YandexInstanceClass{
							Cores:   2,
							Memory:  2048,
							ImageID: "fd-image",
						},
					},
				},
			},
		}

		result := BuildModuleConfigSettingsV2(pcc, ycsettingsv1.ModuleConfigSettings{})

		Expect(result.Nodes.Parameters.ExternalIPAddresses).To(Equal(map[string][]string{
			"master": {"203.0.113.1", "203.0.113.2", "Auto"},
			"worker": {"Auto"},
		}))
		Expect(result.Nodes.Parameters.ExternalSubnetIDs).To(Equal(map[string][]string{
			"master": {"subnet-ext-a", "subnet-ext-b", "subnet-ext-d"},
		}))
	})

	It("skips the master external addressing when there is no master node group", func() {
		pcc := ycpccv1.YandexProviderClusterConfiguration{
			Layout:          "Standard",
			SSHPublicKey:    "ssh-rsa KEY",
			NodeNetworkCIDR: "10.0.0.0/16",
			Provider: ycpccv1.YandexProvider{
				CloudID:  "cloud-1",
				FolderID: "folder-1",
			},
			MasterNodeGroup: ycpccv1.YandexMasterNodeGroup{
				Replicas: 0,
				InstanceClass: ycpccv1.YandexMasterInstanceClass{
					YandexInstanceClass: ycpccv1.YandexInstanceClass{
						ExternalIPAddresses: []string{"203.0.113.1"},
					},
				},
			},
		}

		result := BuildModuleConfigSettingsV2(pcc, ycsettingsv1.ModuleConfigSettings{})

		Expect(result.Nodes.Parameters.ExternalIPAddresses).To(BeEmpty())
	})
})

// The YandexClusterConfiguration schema spells the field etcdDiskSizeGb with a
// lowercase "b" while the YandexInstanceClass CRD uses etcdDiskSizeGB. Decoding
// the PCC with the CRD spelling silently dropped the user's etcd disk size.
var _ = Describe("etcd disk size migration", func() {
	buildMasterInstanceClass := func(pccYAML string) yciccv1.YandexInstanceClass {
		var pcc ycpccv1.YandexProviderClusterConfiguration
		Expect(yaml.Unmarshal([]byte(pccYAML), &pcc)).To(Succeed())

		resources, err := buildMigrationResources(pcc, ycsettingsv1.ModuleConfigSettings{})
		Expect(err).ToNot(HaveOccurred())

		for _, resource := range resources {
			ic, ok := resource.(yciccv1.YandexInstanceClass)
			if ok && ic.Name == cpapi.BuildInstanceClassName("master") {
				return ic
			}
		}

		Fail("master YandexInstanceClass was not built")
		return yciccv1.YandexInstanceClass{}
	}

	It("reads etcdDiskSizeGb from the YandexClusterConfiguration", func() {
		ic := buildMasterInstanceClass(`
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
sshPublicKey: ssh-rsa AAAA
nodeNetworkCIDR: 10.0.0.0/16
masterNodeGroup:
  replicas: 3
  instanceClass:
    cores: 4
    memory: 8192
    imageID: fd-image
    etcdDiskSizeGb: 20
provider:
  cloudID: cloud-1
  folderID: folder-1
  serviceAccountJSON: '{"id":"sa"}'
`)

		Expect(ic.Spec.EtcdDiskSizeGB).To(Equal(ptr.To(20)))
	})

	It("defaults the etcd disk size when the YandexClusterConfiguration omits it", func() {
		ic := buildMasterInstanceClass(`
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
sshPublicKey: ssh-rsa AAAA
nodeNetworkCIDR: 10.0.0.0/16
masterNodeGroup:
  replicas: 3
  instanceClass:
    cores: 4
    memory: 8192
    imageID: fd-image
provider:
  cloudID: cloud-1
  folderID: folder-1
  serviceAccountJSON: '{"id":"sa"}'
`)

		Expect(ic.Spec.EtcdDiskSizeGB).To(Equal(ptr.To(10)))
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
				"module_config":      {mockSnapshot{mustMarshal(mc)}},
				"credential_secrets": {},
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

// --------------------------------------------------------------------------
// d8-migration-resources golden
// --------------------------------------------------------------------------

// The secret payload is what terraform and the in-cluster consumers read after the migration, so
// every projected field is pinned at once. A silent change here recreates master boot and etcd
// disks, which is exactly what the golden is meant to catch.
// wantMigrationResourcesPayload is the expected d8-migration-resources payload, inlined so that
// a review diff shows exactly which projected field moved.
const wantMigrationResourcesPayload = `apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-yandex
stringData:
  authScheme: serviceAccount
  secret: '{"id":"sa-golden"}'
type: cloud-provider.deckhouse.io/credentials
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials-exporter
  namespace: d8-cloud-provider-yandex
stringData:
  authScheme: apiToken
  secret: exporter-golden
type: cloud-provider.deckhouse.io/credentials
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  enabled: true
  settings:
    ccm:
      parameters:
        additionalExternalNetworkIDs:
        - enp-additional
    nodes:
      parameters:
        dhcpOptions:
          domainName: golden.local
          domainNameServers:
          - 10.60.0.2
        externalIPAddresses:
          master:
          - 1.2.3.4
          - 5.6.7.8
          - 9.10.11.12
          worker:
          - 13.14.15.16
          - 17.18.19.20
        externalSubnetIDs:
          master:
          - enp-master
        labels:
          env: golden
        layout: WithNATInstance
        nodeNetworkCIDR: 10.60.0.0/16
        sshPublicKey: ssh-rsa GOLDEN
        withNATInstance:
          internalSubnetCIDR: 10.60.1.0/24
          natInstanceResources: {}
        zones:
        - ru-central1-a
        - ru-central1-b
    provider:
      parameters:
        cloudID: cloud-golden
        folderID: folder-golden
    storage:
      parameters:
        excludedStorageClasses:
        - network-hdd
  version: 2
status: {}
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: master-fc613b4dfd67
    maxPerZone: 3
    minPerZone: 3
    zones:
    - ru-central1-a
  nodeTemplate:
    labels:
      node-role.kubernetes.io/control-plane: ""
      node-role.kubernetes.io/master: ""
  nodeType: CloudPermanent
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: master-fc613b4dfd67
spec:
  additionalLabels:
    role: master
  additionalSubnets:
  - enp-master
  coreFraction: 50
  cores: 8
  diskSizeGB: 60
  diskType: network-ssd
  etcdDiskSizeGB: 20
  imageID: fd8master
  memory: 16384
  networkType: SOFTWARE_ACCELERATED
  platformID: standard-v3
status: {}
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: worker-87eba76e7f31
    maxPerZone: 2
    minPerZone: 2
    zones:
    - ru-central1-b
  nodeTemplate:
    labels:
      node-role/worker: ""
  nodeType: CloudPermanent
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: worker-87eba76e7f31
spec:
  cores: 4
  diskSizeGB: 50
  diskType: network-ssd
  imageID: fd8worker
  memory: 8192
  platformID: standard-v2
status: {}
`

var _ = Describe("migration resources golden", func() {
	It("projects the whole PCC into the migration secret payload", func() {
		pcc := ycpccv1.YandexProviderClusterConfiguration{
			APIVersion:      "deckhouse.io/v1",
			Kind:            "YandexClusterConfiguration",
			Layout:          "WithNATInstance",
			SSHPublicKey:    "ssh-rsa GOLDEN",
			NodeNetworkCIDR: "10.60.0.0/16",
			Zones:           []string{"ru-central1-a", "ru-central1-b"},
			Labels:          map[string]string{"env": "golden"},
			MasterNodeGroup: ycpccv1.YandexMasterNodeGroup{
				Replicas: 3,
				Zones:    []string{"ru-central1-a"},
				InstanceClass: ycpccv1.YandexMasterInstanceClass{
					YandexInstanceClass: ycpccv1.YandexInstanceClass{
						Cores:               8,
						Memory:              16384,
						ImageID:             "fd8master",
						CoreFraction:        ptr.To(50),
						DiskSizeGB:          ptr.To(60),
						DiskType:            ptr.To("network-ssd"),
						Platform:            ptr.To("standard-v3"),
						ExternalIPAddresses: []string{"1.2.3.4", "5.6.7.8", "9.10.11.12"},
						ExternalSubnetIDs:   []string{"enp-master"},
						AdditionalLabels:    map[string]string{"role": "master"},
						NetworkType:         ptr.To("SOFTWARE_ACCELERATED"),
					},
					EtcdDiskSizeGB: ptr.To(20),
				},
			},
			NodeGroups: []ycpccv1.YandexStaticNodeGroup{
				{
					Name:     "worker",
					Replicas: 2,
					Zones:    []string{"ru-central1-b"},
					NodeTemplate: map[string]any{
						"labels": map[string]any{"node-role/worker": ""},
					},
					InstanceClass: ycpccv1.YandexStaticInstanceClass{
						YandexInstanceClass: ycpccv1.YandexInstanceClass{
							Cores:               4,
							Memory:              8192,
							ImageID:             "fd8worker",
							ExternalIPAddresses: []string{"13.14.15.16", "17.18.19.20"},
						},
					},
				},
			},
			Provider: ycpccv1.YandexProvider{
				CloudID:            "cloud-golden",
				FolderID:           "folder-golden",
				ServiceAccountJSON: `{"id":"sa-golden"}`,
			},
			WithNATInstance: &ycpccv1.YandexWithNATInstance{
				InternalSubnetCIDR: ptr.To("10.60.1.0/24"),
				ExporterAPIKey:     ptr.To("exporter-golden"),
			},
			DHCPOptions: &ycpccv1.YandexDHCPOptions{
				DomainName:        ptr.To("golden.local"),
				DomainNameServers: []string{"10.60.0.2"},
			},
		}
		mc := ycsettingsv1.ModuleConfigSettings{
			AdditionalExternalNetworkIDs: []string{"enp-additional"},
			StorageClass: ycsettingsv1.StorageClassSettings{
				Exclude: []string{"network-hdd"},
			},
		}

		resources, err := buildMigrationResources(pcc, mc)
		Expect(err).NotTo(HaveOccurred())

		var payload bytes.Buffer
		for i, resource := range resources {
			if i > 0 {
				payload.WriteString("---\n")
			}
			data, err := yaml.Marshal(resource)
			Expect(err).NotTo(HaveOccurred())
			payload.Write(data)
		}

		Expect(payload.String()).To(Equal(wantMigrationResourcesPayload))
	})
})
