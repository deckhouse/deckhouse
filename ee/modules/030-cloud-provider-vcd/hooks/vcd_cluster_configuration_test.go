/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/base64"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-vcd :: hooks :: vcd_cluster_configuration ::", func() {
	const (
		emptyValues = `
global:
  discovery: {}
cloudProviderVcd:
  internal: {}
`
	)
	var (
		stateACloudDiscoveryData = `
{
   "apiVersion": "deckhouse.io/v1",
   "kind": "VCDCloudProviderDiscoveryData",
   "zones": ["default"]
}
`
		stateAClusterConfiguration1 = `
apiVersion: deckhouse.io/v1
kind: VCDClusterConfiguration
sshPublicKey: "<SSH_PUBLIC_KEY>"
organization: My_Org
virtualDataCenter: My_Org
virtualApplicationName: vapp
layout: Standard
internalNetworkCIDR: 172.16.2.0/24
mainNetwork: internal
masterNodeGroup:
  replicas: 1
  instanceClass:
    template: Templates/ubuntu-focal-20.04
    sizingPolicy: 4cpu8ram
    rootDiskSizeGb: 20
    etcdDiskSizeGb: 20
    storageProfile: nvme
nodeGroups:
- name: worker
  replicas: 1
  instanceClass:
    template: Templates/ubuntu-focal-20.04
    rootDiskSizeGb: 20
    sizingPolicy: 16cpu32ram
    storageProfile: ssd
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
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

	// todo(31337Ghost) eliminate the following dirty hack after `ee` subdirectory will be merged to the root
	// Used to make dhctl config function able to validate `VsphereClusterConfiguration`.
	_ = os.Setenv("DHCTL_CLI_ADDITIONAL_SCHEMAS_PATHS", "/deckhouse/ee/candi")

	a := HookExecutionConfigInit(emptyValues, `{}`)
	Context("Cluster without module configuration", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(notEmptyProviderClusterConfigurationState))
			a.RunHook()
		})

		It("Should fill values from secret", func() {
			Expect(a).To(ExecuteSuccessfully())
			Expect(a.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration").String()).To(MatchYAML(stateAClusterConfiguration1))
			Expect(a.ValuesGet("cloudProviderVcd.internal.providerDiscoveryData").String()).To(MatchJSON(stateACloudDiscoveryData))
		})
	})
})
