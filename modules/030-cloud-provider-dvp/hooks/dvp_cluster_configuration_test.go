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

package hooks

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-dvp :: hooks :: dvp_cluster_configuration ::", func() {
	const (
		emptyValues = `
global:
  discovery: {}
cloudProviderDvp:
  internal: {}
`
	)
	var (
		stateACloudDiscoveryData = `
{
   "apiVersion": "deckhouse.io/v1",
   "kind": "DVPCloudDiscoveryData",
   "zones": ["default"]
}
`
		stateAClusterConfiguration1 = `
apiVersion: deckhouse.io/v1
kind: DVPClusterConfiguration
layout: Standard
masterNodeGroup:
  instanceClass:
    etcdDisk:
      size: 15Gi
      storageClass: ceph-pool-r2-csi-rbd-immediate
    rootDisk:
      image:
        kind: ClusterVirtualImage
        name: ubuntu-2204
      size: 50Gi
      storageClass: ceph-pool-r2-csi-rbd-immediate
    virtualMachine:
      virtualMachineClassName: superbe-class
      bootloader: EFI
      cpu:
        coreFraction: 100%
        cores: 4
      liveMigrationPolicy: PreferForced
      runPolicy: AlwaysOnUnlessStoppedManually
      ipAddresses:
        - Auto
      memory:
        size: 8Gi
  replicas: 3
provider:
  kubeconfigDataBase64: YXBpVmV=
  namespace: cloud-provider01
sshPublicKey: ssh-rsa AAAAB3N
region: ru-msk-1
zones:
- default
`
	)

	notEmptyProviderClusterConfigurationState := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration1)), base64.StdEncoding.EncodeToString([]byte(stateACloudDiscoveryData)))

	a := HookExecutionConfigInit(emptyValues, `{}`)
	Context("Cluster without module configuration", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(notEmptyProviderClusterConfigurationState))
			a.RunHook()
		})

		It("Should fill values from secret", func() {
			Expect(a).To(ExecuteSuccessfully())
			Expect(a.ValuesGet("cloudProviderDvp.internal.providerClusterConfiguration").String()).To(MatchYAML(stateAClusterConfiguration1))
			Expect(a.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData").String()).To(MatchJSON(stateACloudDiscoveryData))
		})
	})
})
