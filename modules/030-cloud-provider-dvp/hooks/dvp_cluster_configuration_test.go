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

package hooks

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func GenerateSecretYAML(clusterConfiguration, discoveryData string) string {
	return fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
 name: d8-provider-cluster-configuration
 namespace: kube-system
data:
 "cloud-provider-cluster-configuration.yaml": %s
 "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(clusterConfiguration)), base64.StdEncoding.EncodeToString([]byte(discoveryData)))
}

var _ = Describe("Modules :: cloud-provider-dvp :: hooks :: dvp_cluster_configuration ::", func() {
	const (
		initValuesString = `
global:
  discovery: {}
cloudProviderDvp:
  internal: {}
`
	)

	// correct ClusterConfiguration
	var stateCorrectDVPClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: DVPClusterConfiguration
layout: Standard
sshPublicKey: "ssh-rsa AAA"
masterNodeGroup:
  replicas: 1
  instanceClass:
    virtualMachine:
      cpu:
        cores: 2
        coreFraction: 100%
      memory:
        size: 2Gi
    rootDisk:
      size: 10Gi
      image:
        kind: ClusterVirtualImage
        name: image-name
    etcdDisk:
      size: 10Gi
provider:
  kubeconfigDataBase64: ZXhhbXBsZQo=
  namespace: tenant
`

	// wrong ClusterConfiguration
	stateWrongDVPClusterConfiguration := `
apiVersion: deckhouse.io/v1
kind: DVPClusterConfiguration
layout: WithNATInstance
`

	var stateCorrectDiscoveryData = `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "DVPCloudDiscoveryData",
  "storageClasses":[
    {"name":"a"},
    {"name":"b","isDefault":true}
		]
}
`

	var stateWrongDiscoveryData = `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "DVPCloudDiscoveryData",
  "zones": [{"name":"a"},{"name":"b"}]
}
`

	var secretStateCorrectClusterCorrectData = GenerateSecretYAML(stateCorrectDVPClusterConfiguration, stateCorrectDiscoveryData)
	var secretStateCorrectClusterWrongData = GenerateSecretYAML(stateCorrectDVPClusterConfiguration, stateWrongDiscoveryData)
	var secretStateWrongClusterWrongData = GenerateSecretYAML(stateWrongDVPClusterConfiguration, stateWrongDiscoveryData)

	a := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Cluster has empty state", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(``))
			a.RunHook()
		})

		It("Hook should fail with errors", func() {
			Expect(a).To(Not(ExecuteSuccessfully()))
			Expect(a.GoHookError.Error()).Should(ContainSubstring(`kube-system/d8-provider-cluster-configuration`))
		})
	})

	b := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Correct DVP Cluster Configuration and correct discovery data", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(secretStateCorrectClusterCorrectData))
			b.RunHook()
		})

		It("All values should be ok", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderDvp.internal.providerClusterConfiguration").String()).To(MatchYAML(stateCorrectDVPClusterConfiguration))
			Expect(b.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData").String()).To(MatchYAML(`
apiVersion: deckhouse.io/v1
kind: DVPCloudDiscoveryData
storageClasses:
  - name: a
  - name: b
    isDefault: true`))
		})
	})

	c := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Correct DVP Cluster Configuration and wrong discovery data", func() {
		BeforeEach(func() {
			c.BindingContexts.Set(b.KubeStateSet(secretStateCorrectClusterWrongData))
			c.RunHook()
		})

		It("Hook should fail with errors", func() {
			Expect(c).To(Not(ExecuteSuccessfully()))
			Expect(c.GoHookError.Error()).Should(ContainSubstring(`.storageClasses is required`))
		})
	})

	d := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Wrong DVP Cluster Configuration", func() {
		BeforeEach(func() {
			d.BindingContexts.Set(b.KubeStateSet(secretStateWrongClusterWrongData))
			d.RunHook()
		})

		It("Hook should fail with errors", func() {
			Expect(d).To(Not(ExecuteSuccessfully()))
			Expect(d.GoHookError.Error()).Should(ContainSubstring(`layout should be one of [Standard]`))
			Expect(d.GoHookError.Error()).Should(ContainSubstring(`.masterNodeGroup is required`))
			Expect(d.GoHookError.Error()).Should(ContainSubstring(`.sshPublicKey is required`))
			Expect(d.GoHookError.Error()).Should(ContainSubstring(`.provider is required`))
		})
	})

})
