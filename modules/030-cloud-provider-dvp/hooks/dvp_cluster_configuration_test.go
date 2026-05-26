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
	a.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
	a.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
	a.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	Context("Cluster without module configuration", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(notEmptyProviderClusterConfigurationState))
			a.RunHook()
		})

		It("Should fill values from secret", func() {
			Expect(a).To(ExecuteSuccessfully())
			Expect(a.ValuesGet("cloudProviderDvp.internal.providerClusterConfiguration").String()).To(MatchYAML(stateAClusterConfiguration1))
			Expect(a.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData").String()).To(MatchJSON(stateACloudDiscoveryData))

			moduleConfig := a.KubernetesGlobalResource("ModuleConfig", "cloud-provider-dvp")
			Expect(moduleConfig.Exists()).To(BeFalse())

			credentialSecret := a.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-cloud-provider-dvp-credentials")
			Expect(credentialSecret.Exists()).To(BeFalse())

			masterInstanceClass := a.KubernetesGlobalResource("DVPInstanceClass", "master-dvp")
			Expect(masterInstanceClass.Exists()).To(BeFalse())

			masterNodeGroup := a.KubernetesGlobalResource("NodeGroup", "master")
			Expect(masterNodeGroup.Exists()).To(BeFalse())

			migrationResourcesSecret := a.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-migration-resources")
			Expect(migrationResourcesSecret.Exists()).To(BeTrue())
			resourcesManifest, err := base64.StdEncoding.DecodeString(migrationResourcesSecret.Field(`data.resources\.yaml`).String())
			Expect(err).ToNot(HaveOccurred())
			Expect(string(resourcesManifest)).To(MatchYAML(`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  enabled: true
  version: 2
  settings:
    provider:
      parameters:
        namespace: cloud-provider01
    storage:
      enabled: true
      parameters: {}
    nodes:
      enabled: true
      parameters:
        layout: Standard
        sshPublicKey: ssh-rsa AAAAB3N
        region: ru-msk-1
        zones:
        - default
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-provider-dvp-credentials
  namespace: d8-cloud-provider-dvp
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
type: cloud-provider.deckhouse.io/credentials
data:
  authScheme: S3ViZWNvbmZpZw==
  secret: YXBpVmV=
---
apiVersion: deckhouse.io/v1alpha1
kind: DVPInstanceClass
metadata:
  name: master-dvp
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
spec:
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
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
spec:
  nodeType: CloudPermanent
  cloudInstances:
    zones:
    - default
    minPerZone: 3
    maxPerZone: 3
    classReference:
      kind: DVPInstanceClass
      name: master-dvp
  nodeTemplate:
    labels:
      node-role.kubernetes.io/control-plane: ""
      node-role.kubernetes.io/master: ""
`))
		})
	})
})
