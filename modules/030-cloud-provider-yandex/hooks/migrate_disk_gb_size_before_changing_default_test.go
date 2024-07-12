/*
Copyright 2024 Flant JSC

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
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: migrate_disk_gb_size_before_changing_default ::", func() {
	const (
		initValuesString = `
global:
  discovery: {}
cloudProviderYandex:
  internal: {}
`
	)

	generateProviderSecret := func(pcc string) string {
		stateCloudDiscoveryData := base64.StdEncoding.EncodeToString([]byte(`
{
  "apiVersion": "deckhouse.io/v1",
  "defaultLbTargetGroupNetworkId": "test",
  "internalNetworkIDs": [
    "test"
  ],
  "kind": "YandexCloudDiscoveryData",
  "region": "test",
  "routeTableID": "test",
  "shouldAssignPublicIPAddress": false,
  "zoneToSubnetIdMap": {
    "ru-central1-a": "test",
    "ru-central1-b": "test",
    "ru-central1-c": "test"
  },
  "zones": [
    "ru-central1-a",
    "ru-central1-b",
    "ru-central1-c"
  ]
}
`))
		return fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(pcc)), stateCloudDiscoveryData)
	}

	installCM162 := `
apiVersion: v1
data:
  version: v1.62.0
kind: ConfigMap
metadata:
  name: install-data
  namespace: d8-system
	`

	installCM161 := `
apiVersion: v1
data:
  version: v1.61.1
kind: ConfigMap
metadata:
  name: install-data
  namespace: d8-system
`
	installCMDev := `
apiVersion: v1
data:
  version: dev
kind: ConfigMap
metadata:
  name: install-data
  namespace: d8-system
`

	assertSetOldDiskSizeForMasterNG := func(f *HookExecutionConfig) {
		s := f.KubernetesResource("Secret", "kube-system", "d8-provider-cluster-configuration")
		Expect(s.Exists()).To(BeTrue())

		clusterConfig := s.Field("data.cloud-provider-cluster-configuration\\.yaml").String()
		config, err := base64.StdEncoding.DecodeString(clusterConfig)
		Expect(err).Should(BeNil())

		type ic struct {
			DiskSizeGB int `json:"diskSizeGB"`
		}

		type masterNg struct {
			InstanceClass ic `json:"instanceClass"`
		}

		type conf struct {
			MasterNodeGroup masterNg `json:"masterNodeGroup"`
		}

		var p conf
		err = yaml.Unmarshal(config, &p)
		Expect(err).Should(BeNil())

		Expect(p.MasterNodeGroup.InstanceClass.DiskSizeGB).Should(Equal(20))
	}

	assertDiskSizeForOtherNG := func(f *HookExecutionConfig, ngs map[string]int) {
		s := f.KubernetesResource("Secret", "kube-system", "d8-provider-cluster-configuration")
		Expect(s.Exists()).To(BeTrue())

		clusterConfig := s.Field("data.cloud-provider-cluster-configuration\\.yaml").String()
		config, err := base64.StdEncoding.DecodeString(clusterConfig)
		Expect(err).Should(BeNil())

		type ic struct {
			DiskSizeGB int `json:"diskSizeGB"`
		}

		type ng struct {
			InstanceClass ic     `json:"instanceClass"`
			Name          string `json:"name"`
		}

		type conf struct {
			NGS []ng `json:"nodeGroups"`
		}

		var p conf
		err = yaml.Unmarshal(config, &p)
		Expect(err).Should(BeNil())

		expectedNgs := make(map[string]int)
		for _, ng := range p.NGS {
			expectedNgs[ng.Name] = ng.InstanceClass.DiskSizeGB
		}

		for ng, size := range ngs {
			Expect(expectedNgs).To(HaveKey(ng))
			Expect(expectedNgs[ng]).To(Equal(size))
		}
	}

	assertSaveBackupBeforeMigrate := func(f *HookExecutionConfig, pccs string) {
		s := f.KubernetesResource("Secret", "kube-system", "d8-provider-cluster-configuration-bkp-disk-gb")

		if pccs != "" {
			Expect(s.Exists()).To(BeTrue())

			var secret v1.Secret

			err := yaml.Unmarshal([]byte(pccs), &secret)
			Expect(err).Should(BeNil())

			clusterConfig := s.Field("data.cloud-provider-cluster-configuration\\.yaml").String()
			data := s.Field("data.cloud-provider-discovery-data\\.json").String()

			Expect(clusterConfig).To(Equal(base64.StdEncoding.EncodeToString(secret.Data["cloud-provider-cluster-configuration.yaml"])))
			Expect(data).To(Equal(base64.StdEncoding.EncodeToString(secret.Data["cloud-provider-discovery-data.json"])))

			return
		}

		Expect(s.Exists()).To(BeFalse())
	}

	assertNoChangeSecret := func(f *HookExecutionConfig, pccs string) {
		s := f.KubernetesResource("Secret", "kube-system", "d8-provider-cluster-configuration")

		Expect(s.Exists()).To(BeTrue())
		Expect(s.ToYaml()).To(MatchYAML(pccs))
	}

	f := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Cluster has empty state", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook should not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster has provider cluster configuration secret without diskSizeGB in master nodegroup", func() {
		const pcc = `
apiVersion: deckhouse.io/v1
existingNetworkID: enpma5uvcfbkuac1i1jb
kind: YandexClusterConfiguration
layout: WithNATInstance
masterNodeGroup:
  instanceClass:
    cores: 2
    etcdDiskSizeGb: 10
    imageID: test
    memory: 4096
    platform: standard-v2
  replicas: 1
provider:
  cloudID: test
  folderID: test
  serviceAccountJSON: |-
    {
      "id": "test"
    }
withNATInstance:
  internalSubnetID: test
  natInstanceExternalAddress: 84.201.160.148
  exporterAPIKey: ""
  natInstanceResources:
    cores: 2
    memory: 2048
nodeNetworkCIDR: 84.201.160.148/31
sshPublicKey: ssh-rsa AAAAAbbbb
`
		Context("Cluster has install data config with version >= 1.62", func() {
			var pccs = generateProviderSecret(pcc)
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(pccs + "\n---\n" + installCM162))
				f.RunHook()
			})

			It("Hook should execute successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should not change provider configuration secret", func() {
				assertNoChangeSecret(f, pccs)
				assertSaveBackupBeforeMigrate(f, "")
			})
		})

		Context("Cluster has install data config with version < 1.62", func() {
			var pccs = generateProviderSecret(pcc)
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(pccs + "\n---\n" + installCM161))
				f.RunHook()
			})

			It("Hook should execute successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should set diskSizeGB for old default 20", func() {
				assertSetOldDiskSizeForMasterNG(f)
				assertSaveBackupBeforeMigrate(f, pccs)
			})
		})

		Context("Cluster has install data config with 'dev' version", func() {
			var pccs = generateProviderSecret(pcc)
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(pccs + "\n---\n" + installCMDev))
				f.RunHook()
			})

			It("Hook should execute successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should set diskSizeGB for old default 20", func() {
				assertSetOldDiskSizeForMasterNG(f)
				assertSaveBackupBeforeMigrate(f, pccs)
			})
		})
	})

	Context("Cluster has provider cluster configuration secret with diskSizeGB in master nodegroup", func() {
		const pcc = `
apiVersion: deckhouse.io/v1
existingNetworkID: enpma5uvcfbkuac1i1jb
kind: YandexClusterConfiguration
layout: WithNATInstance
masterNodeGroup:
  instanceClass:
    cores: 2
    etcdDiskSizeGb: 10
    imageID: test
    memory: 4096
    platform: standard-v2
    diskSizeGB: 35
  replicas: 1
provider:
  cloudID: test
  folderID: test
  serviceAccountJSON: |-
    {
      "id": "test"
    }
withNATInstance:
  internalSubnetID: test
  natInstanceExternalAddress: 84.201.160.148
  exporterAPIKey: ""
  natInstanceResources:
    cores: 2
    memory: 2048
nodeNetworkCIDR: 84.201.160.148/31
sshPublicKey: ssh-rsa AAAAAbbbb
`
		var pccs = generateProviderSecret(pcc)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pccs))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should not change secret", func() {
			assertNoChangeSecret(f, pccs)
			assertSaveBackupBeforeMigrate(f, "")
		})
	})
	Context("Cluster has provider cluster configuration secret with diskSizeGB another node group", func() {
		const pcc = `
apiVersion: deckhouse.io/v1
existingNetworkID: enpma5uvcfbkuac1i1jb
kind: YandexClusterConfiguration
layout: WithNATInstance
masterNodeGroup:
  instanceClass:
    cores: 2
    etcdDiskSizeGb: 10
    imageID: test
    memory: 4096
    platform: standard-v2
    diskSizeGB: 35
  replicas: 1
nodeGroups:
- name: khm
  replicas: 0
  instanceClass:
    externalIPAddresses:
    - Auto
    cores: 2
    memory: 4096
    imageID: fd8vqk0bcfhn31stn2ts
    coreFraction: 50
    diskSizeGB: 35
provider:
  cloudID: test
  folderID: test
  serviceAccountJSON: |-
    {
      "id": "test"
    }
withNATInstance:
  internalSubnetID: test
  natInstanceExternalAddress: 84.201.160.148
  exporterAPIKey: ""
  natInstanceResources:
    cores: 2
    memory: 2048
nodeNetworkCIDR: 84.201.160.148/31
sshPublicKey: ssh-rsa AAAAAbbbb
`
		var pccs = generateProviderSecret(pcc)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pccs))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should not change secret", func() {
			assertNoChangeSecret(f, pccs)
			assertSaveBackupBeforeMigrate(f, "")
		})
	})

	Context("Cluster has provider cluster configuration secret without diskSizeGB in others node group", func() {
		const pcc = `
apiVersion: deckhouse.io/v1
existingNetworkID: enpma5uvcfbkuac1i1jb
kind: YandexClusterConfiguration
layout: WithNATInstance
masterNodeGroup:
  instanceClass:
    cores: 2
    etcdDiskSizeGb: 10
    imageID: test
    memory: 4096
    platform: standard-v2
    diskSizeGB: 35
  replicas: 1
nodeGroups:
- name: khm
  replicas: 0
  instanceClass:
    externalIPAddresses:
    - Auto
    cores: 2
    memory: 4096
    imageID: fd8vqk0bcfhn31stn2ts
    coreFraction: 50
- name: mhk
  replicas: 0
  instanceClass:
    externalIPAddresses:
    - Auto
    cores: 2
    memory: 4096
    imageID: fd8vqk0bcfhn31stn2ts
    coreFraction: 50
provider:
  cloudID: test
  folderID: test
  serviceAccountJSON: |-
    {
      "id": "test"
    }
withNATInstance:
  internalSubnetID: test
  natInstanceExternalAddress: 84.201.160.148
  exporterAPIKey: ""
  natInstanceResources:
    cores: 2
    memory: 2048
nodeNetworkCIDR: 84.201.160.148/31
sshPublicKey: ssh-rsa AAAAAbbbb
`
		var pccs = generateProviderSecret(pcc)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pccs))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should old default size 20 for all nodegroup", func() {
			assertDiskSizeForOtherNG(f, map[string]int{
				"khm": 20,
				"mhk": 20,
			})
			assertSaveBackupBeforeMigrate(f, pccs)
		})
	})

	Context("Cluster has provider cluster configuration secret with and without diskSizeGB in others node group", func() {
		const pcc = `
apiVersion: deckhouse.io/v1
existingNetworkID: enpma5uvcfbkuac1i1jb
kind: YandexClusterConfiguration
layout: WithNATInstance
masterNodeGroup:
  instanceClass:
    cores: 2
    etcdDiskSizeGb: 10
    imageID: test
    memory: 4096
    platform: standard-v2
    diskSizeGB: 35
  replicas: 1
nodeGroups:
- name: khm
  replicas: 0
  instanceClass:
    externalIPAddresses:
    - Auto
    cores: 2
    memory: 4096
    imageID: fd8vqk0bcfhn31stn2ts
    coreFraction: 50
    diskSizeGB: 35
- name: mhk
  replicas: 0
  instanceClass:
    externalIPAddresses:
    - Auto
    cores: 2
    memory: 4096
    imageID: fd8vqk0bcfhn31stn2ts
    coreFraction: 50
provider:
  cloudID: test
  folderID: test
  serviceAccountJSON: |-
    {
      "id": "test"
    }
withNATInstance:
  internalSubnetID: test
  natInstanceExternalAddress: 84.201.160.148
  exporterAPIKey: ""
  natInstanceResources:
    cores: 2
    memory: 2048
nodeNetworkCIDR: 84.201.160.148/31
sshPublicKey: ssh-rsa AAAAAbbbb
`
		var pccs = generateProviderSecret(pcc)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pccs))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should old default size 20 for nodegroup without diskSize and not change with diskSize", func() {
			assertDiskSizeForOtherNG(f, map[string]int{
				"khm": 35,
				"mhk": 20,
			})

			assertSaveBackupBeforeMigrate(f, pccs)
		})
	})

	Context("Cluster has provider cluster configuration secret with diskSizeGB in others node group", func() {
		const pcc = `
apiVersion: deckhouse.io/v1
existingNetworkID: enpma5uvcfbkuac1i1jb
kind: YandexClusterConfiguration
layout: WithNATInstance
masterNodeGroup:
  instanceClass:
    cores: 2
    etcdDiskSizeGb: 10
    imageID: test
    memory: 4096
    platform: standard-v2
    diskSizeGB: 35
  replicas: 1
nodeGroups:
- name: khm
  replicas: 0
  instanceClass:
    externalIPAddresses:
    - Auto
    cores: 2
    memory: 4096
    imageID: fd8vqk0bcfhn31stn2ts
    coreFraction: 50
    diskSizeGB: 35
- name: mhk
  replicas: 0
  instanceClass:
    externalIPAddresses:
    - Auto
    cores: 2
    memory: 4096
    imageID: fd8vqk0bcfhn31stn2ts
    coreFraction: 50
    diskSizeGB: 45
provider:
  cloudID: test
  folderID: test
  serviceAccountJSON: |-
    {
      "id": "test"
    }
withNATInstance:
  internalSubnetID: test
  natInstanceExternalAddress: 84.201.160.148
  exporterAPIKey: ""
  natInstanceResources:
    cores: 2
    memory: 2048
nodeNetworkCIDR: 84.201.160.148/31
sshPublicKey: ssh-rsa AAAAAbbbb
`
		var pccs = generateProviderSecret(pcc)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pccs))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should old default size 20 for nodegroup without diskSize and not change with diskSize", func() {
			assertNoChangeSecret(f, pccs)
			assertSaveBackupBeforeMigrate(f, "")
		})
	})

	Context("Cluster has provider cluster configuration secret without diskSizeGB in others node group and master ng both", func() {
		const pcc = `
apiVersion: deckhouse.io/v1
existingNetworkID: enpma5uvcfbkuac1i1jb
kind: YandexClusterConfiguration
layout: WithNATInstance
masterNodeGroup:
  instanceClass:
    cores: 2
    etcdDiskSizeGb: 10
    imageID: test
    memory: 4096
    platform: standard-v2
  replicas: 1
nodeGroups:
- name: khm
  replicas: 0
  instanceClass:
    externalIPAddresses:
    - Auto
    cores: 2
    memory: 4096
    imageID: fd8vqk0bcfhn31stn2ts
    coreFraction: 50
provider:
  cloudID: test
  folderID: test
  serviceAccountJSON: |-
    {
      "id": "test"
    }
withNATInstance:
  internalSubnetID: test
  natInstanceExternalAddress: 84.201.160.148
  exporterAPIKey: ""
  natInstanceResources:
    cores: 2
    memory: 2048
nodeNetworkCIDR: 84.201.160.148/31
sshPublicKey: ssh-rsa AAAAAbbbb
`
		var pccs = generateProviderSecret(pcc)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pccs))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should old default size 20 for nodegroup and master nodegroup", func() {
			assertSetOldDiskSizeForMasterNG(f)
			assertDiskSizeForOtherNG(f, map[string]int{
				"khm": 20,
			})
			assertSaveBackupBeforeMigrate(f, pccs)
		})
	})
})
