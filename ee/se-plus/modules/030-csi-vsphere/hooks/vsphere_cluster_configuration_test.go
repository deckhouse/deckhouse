/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: csi-vsphere :: hooks :: vsphere_cluster_configuration ::", func() {
	const (
		emptyValues = `
global:
  discovery: {}
csiVsphere:
  internal: {}
`
		filledValues = `
global:
  discovery: {}
csiVsphere:
  internal: {}
  host: override
  username: override
  password: override
  insecure: true
  regionTagCategory: override
  zoneTagCategory: override
  disableTimesync: false
  sshKeys: [override1, override2]
  region: override
  zones: [override1, override2]
`
		filledValuesWithoutSomeFields = `
global:
  discovery: {}
csiVsphere:
  internal: {}
  host: override
  username: override
  password: override
  insecure: true
  disableTimesync: false
  sshKeys: [override1, override2]
  region: override
`

		stateAClusterConfiguration = `
disableTimesync: false
provider:
  insecure: true
  password: override
  server: override
  username: override
region: override
regionTagCategory: override
sshPublicKey: override1
vmFolderExists: false
zoneTagCategory: override
zones:
- override1
- override2
`

		emptyProviderClusterConfigurationState = `
apiVersion: v1
kind: Secret
metadata:
 name: d8-provider-cluster-configuration
 namespace: kube-system
data: {}
`
	)
	// todo(31337Ghost) eliminate the following dirty hack after `ee` subdirectory will be merged to the root
	// Used to make dhctl config function able to validate `VsphereClusterConfiguration`.
	_ = os.Setenv("DHCTL_CLI_ADDITIONAL_SCHEMAS_PATHS", "/deckhouse/ee/se-plus/candi")
	a := HookExecutionConfigInit(filledValues, `{}`)
	Context("Cluster with module configuration without zones, with empty secret", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(emptyProviderClusterConfigurationState))
			a.RunHook()
		})

		It("Should fill values from module configuration", func() {
			Expect(a).To(ExecuteSuccessfully())
			Expect(a.ValuesGet("csiVsphere.internal.providerClusterConfiguration").String()).To(MatchYAML(stateAClusterConfiguration))
			Expect(a.ValuesGet("csiVsphere.internal.providerDiscoveryData").String()).To(MatchJSON("{}"))
		})
	})
	d := HookExecutionConfigInit(filledValuesWithoutSomeFields, `{}`)
	Context("Cluster with module configuration without zones, with empty secret", func() {
		BeforeEach(func() {
			d.BindingContexts.Set(d.KubeStateSet(emptyProviderClusterConfigurationState))
			d.RunHook()
		})

		It("Should fail", func() {
			Expect(d).To(Not(ExecuteSuccessfully()))
		})
	})
})
