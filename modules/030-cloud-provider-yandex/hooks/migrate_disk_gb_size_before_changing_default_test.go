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
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(pcc)), stateCloudDiscoveryData)
	}

	installCM161 := `
apiVersion: v1
data:
  version: v1.61.4
kind: ConfigMap
metadata:
  name: install-data
  namespace: d8-system
	`

	installCM160 := `
apiVersion: v1
data:
  version: v1.60.1
kind: ConfigMap
metadata:
  name: install-data
  namespace: d8-system
`

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
		Context("Cluster has install data config with version >= 1.61", func() {
			var pccs = generateProviderSecret(pcc)
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(pccs + "\n---\n" + installCM161))
				f.RunHook()
			})

			It("Hook should execute successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should not change provider configuration secret", func() {
				s := f.KubernetesResource("Secret", "kube-system", "d8-cluster-configuration")

				Expect(s.Exists()).To(BeTrue())
				Expect(s.ToYaml()).To(MatchYAML(pccs))
			})
		})

		FContext("Cluster has install data config with version < 1.60", func() {
			var pccs = generateProviderSecret(pcc)
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(pccs + "\n---\n" + installCM160))
				f.RunHook()
			})

			It("Hook should execute successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Hook should not change provider configuration secret", func() {
				s := f.KubernetesResource("Secret", "kube-system", "d8-cluster-configuration")

				Expect(s.Exists()).To(BeTrue())
				Expect(s.ToYaml()).To(MatchYAML(pccs))
			})
		})
	})
})
