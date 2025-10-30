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

var _ = Describe("Modules :: cloud-provider-aws :: hooks :: aws_cluster_configuration ::", func() {
	const (
		initValuesString = `
global:
  discovery: {}
cloudProviderAws:
  internal: {}
`
	)

	var (
		// correct cdd
		stateBCloudDiscoveryData = `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "AWSCloudDiscoveryData",
  "instances": {
    "iamProfileName": "zzz-node",
    "additionalSecurityGroups": [
      "sg-zzz",
      "sg-qqq"
    ],
    "ami": "ami-aaabbbccc",
    "associatePublicIPAddress": true
  },
  "zones": ["zz-zzz-1z", "xx-xxx-1x", "cc-ccc-1c"],
  "zoneToSubnetIdMap": {
    "zzz": "xxx"
  },
  "loadBalancerSecurityGroup": "sg-lbzzz",
  "keyName": "kzzz"
}`

		// wrong cdd
		stateCCloudDiscoveryData = `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "AWSCloudDiscoveryData",
  "instances": {
    "additionalSecurityGroups": [
      "wrongsgname"
    ]
  }
}`

		// correct cc
		stateBClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
layout: Standard
masterNodeGroup:
  instanceClass:
    ami: ami-03818140b4ac9ae2b
    instanceType: t2.medium
  replicas: 1
nodeGroups:
- instanceClass:
    ami: ami-03818140b4ac9ae2b
    instanceType: t2.medium
  name: qqq
  nodeTemplate:
    labels:
      node-role.kubernetes.io/qqq: ""
  replicas: 1
vpcNetworkCIDR: 10.222.0.0/16
provider:
  providerAccessKeyId: keyzzz
  providerSecretAccessKey: secretzzz
  region: eu-zzz
sshPublicKey: kekekey
`

		// wrong cc
		stateDClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
vpcNetworkCIDR: 1.1.1.1.1.1/16
`

		stateB = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateBClusterConfiguration)), base64.StdEncoding.EncodeToString([]byte(stateBCloudDiscoveryData)))

		stateC = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateBClusterConfiguration)), base64.StdEncoding.EncodeToString([]byte(stateCCloudDiscoveryData)))

		stateD = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateDClusterConfiguration)), base64.StdEncoding.EncodeToString([]byte(stateBCloudDiscoveryData)))
	)

	a := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Cluster has empty state", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(``))
			a.RunHook()
		})

		It("Hook should fail with errors", func() {
			Expect(a).To(Not(ExecuteSuccessfully()))
			Expect(a.GoHookError.Error()).Should(ContainSubstring(`can't find Secret d8-provider-cluster-configuration in Namespace kube-system`))
		})
	})

	b := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Provider data is successfully discovered", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(stateB))
			b.RunHook()
		})

		It("All values should be gathered from discovered data", func() {
			Expect(b).To(ExecuteSuccessfully())

			Expect(b.ValuesGet("cloudProviderAws.internal.region").String()).To(Equal("eu-zzz"))
			Expect(b.ValuesGet("cloudProviderAws.internal.providerAccessKeyId").String()).To(Equal("keyzzz"))
			Expect(b.ValuesGet("cloudProviderAws.internal.providerSecretAccessKey").String()).To(Equal("secretzzz"))
			Expect(b.ValuesGet("cloudProviderAws.internal.zones").String()).To(MatchJSON(`["zz-zzz-1z", "xx-xxx-1x", "cc-ccc-1c"]`))
			Expect(b.ValuesGet("cloudProviderAws.internal.zoneToSubnetIdMap").String()).To(MatchJSON(`{"zzz":"xxx"}`))
			Expect(b.ValuesGet("cloudProviderAws.internal.loadBalancerSecurityGroup").String()).To(Equal("sg-lbzzz"))
			Expect(b.ValuesGet("cloudProviderAws.internal.keyName").String()).To(Equal("kzzz"))
			Expect(b.ValuesGet("cloudProviderAws.internal.instances").String()).To(MatchJSON(`{"ami":"ami-aaabbbccc","associatePublicIPAddress": true,"iamProfileName":"zzz-node","additionalSecurityGroups":["sg-zzz","sg-qqq"]}`))
		})
	})

	c := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Discovery data is wrong", func() {
		BeforeEach(func() {
			c.BindingContexts.Set(c.KubeStateSet(stateC))
			c.RunHook()
		})

		It("All values should be gathered from discovered data", func() {
			Expect(c).To(Not(ExecuteSuccessfully()))

			Expect(c.GoHookError.Error()).Should(ContainSubstring(`instances.additionalSecurityGroups in body should match`))
			Expect(c.GoHookError.Error()).Should(ContainSubstring(`instances.ami in body is required`))
			Expect(c.GoHookError.Error()).Should(ContainSubstring(`instances.associatePublicIPAddress in body is required`))
			Expect(c.GoHookError.Error()).Should(ContainSubstring(`instances.iamProfileName in body is required`))
			Expect(c.GoHookError.Error()).Should(ContainSubstring(`keyName in body is required`))
			Expect(c.GoHookError.Error()).Should(ContainSubstring(`loadBalancerSecurityGroup in body is required`))
			Expect(c.GoHookError.Error()).Should(ContainSubstring(`zoneToSubnetIdMap in body is required`))
			Expect(c.GoHookError.Error()).Should(ContainSubstring(`zones in body is required`))
		})
	})

	d := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Discovery data is wrong", func() {
		BeforeEach(func() {
			d.BindingContexts.Set(d.KubeStateSet(stateD))
			d.RunHook()
		})

		It("All values should be gathered from discovered data", func() {
			Expect(d).To(Not(ExecuteSuccessfully()))

			Expect(d.GoHookError.Error()).Should(ContainSubstring(`layout in body is required`))
			Expect(d.GoHookError.Error()).Should(ContainSubstring(`vpcNetworkCIDR in body should match`))
			Expect(d.GoHookError.Error()).Should(ContainSubstring(`provider in body is required`))
			Expect(d.GoHookError.Error()).Should(ContainSubstring(`masterNodeGroup in body is required`))
			Expect(d.GoHookError.Error()).Should(ContainSubstring(`sshPublicKey in body is required`))
		})
	})
})
