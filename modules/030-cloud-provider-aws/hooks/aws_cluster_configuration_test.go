package hooks

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

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
		stateA = `
apiVersion: v1
kind: Secret
metadata:
 name: d8-provider-cluster-configuration
 namespace: kube-system
data: {}
`

		stateBCloudDiscoveryData = `
{
  "instances": {
    "iamProfileName": "zzz-node",
    "additionalSecurityGroups": [
      "sg-zzz",
      "sg-qqq"
    ]
  },
  "zones": ["z", "x", "c"],
  "zoneToSubnetIdMap": {
    "zzz": "xxx"
  },
  "loadBalancerSecurityGroup": "sg-lb-zzz",
  "keyName": "kzzz"
}`
		stateBClusterConfiguration = `
apiVersion: deckhouse.io/v1alpha1
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
standard:
  associatePublicIPToMasters: true
sshPublicKey: kekekey
`

		stateB = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateBClusterConfiguration)), base64.StdEncoding.EncodeToString([]byte(stateBCloudDiscoveryData)))
	)

	a := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Cluster has empty discovery data", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(stateA))
			a.RunHook()
		})

		It("Hook should fail with errors", func() {
			Expect(a).To(Not(ExecuteSuccessfully()))

			Expect(a.Session.Err).Should(gbytes.Say(`ERROR: region is not configured in kube-system/d8-provider-cluster-configuration Secret`))
			Expect(a.Session.Err).Should(gbytes.Say(`ERROR: providerAccessKeyId is not configured in kube-system/d8-provider-cluster-configuration Secret`))
			Expect(a.Session.Err).Should(gbytes.Say(`ERROR: providerSecretAccessKey is not configured in kube-system/d8-provider-cluster-configuration Secret`))
		})
	})

	c := HookExecutionConfigInit(initValuesString, `{}`)
	Context("Provider data is discovered", func() {
		BeforeEach(func() {
			c.BindingContexts.Set(c.KubeStateSet(stateB))
			c.RunHook()
		})

		It("All values should be gathered from discovered data", func() {
			Expect(c).To(ExecuteSuccessfully())

			Expect(c.ValuesGet("cloudProviderAws.internal.region").String()).To(Equal("eu-zzz"))
			Expect(c.ValuesGet("cloudProviderAws.internal.providerAccessKeyId").String()).To(Equal("keyzzz"))
			Expect(c.ValuesGet("cloudProviderAws.internal.providerSecretAccessKey").String()).To(Equal("secretzzz"))
			Expect(c.ValuesGet("cloudProviderAws.internal.zones").String()).To(MatchJSON(`["z", "x", "c"]`))
			Expect(c.ValuesGet("cloudProviderAws.internal.zoneToSubnetIdMap").String()).To(MatchJSON(`{"zzz":"xxx"}`))
			Expect(c.ValuesGet("cloudProviderAws.internal.loadBalancerSecurityGroup").String()).To(Equal("sg-lb-zzz"))
			Expect(c.ValuesGet("cloudProviderAws.internal.keyName").String()).To(Equal("kzzz"))
			Expect(c.ValuesGet("cloudProviderAws.internal.instances").String()).To(MatchJSON(`{"iamProfileName":"zzz-node","additionalSecurityGroups":["sg-zzz","sg-qqq"]}`))
		})
	})
})
