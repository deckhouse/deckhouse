/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString       = `{"global": {}, "controlPlaneManager":{"internal":{}}}`
	initConfigValuesString = `{}`
)

func d8ClusterConfigurationSecretData(version string) string {
	var secretDataTemplate = `apiVersion: deckhouse.io/v1
cloud:
  prefix: prefix
  provider: OpenStack
clusterDomain: cluster.local
clusterType: Cloud
defaultCRI: Containerd
kind: ClusterConfiguration
kubernetesVersion: "%s"
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
`

	resultStr := fmt.Sprintf(secretDataTemplate, version)
	return base64.StdEncoding.EncodeToString([]byte(resultStr))
}

var _ = Describe("Module hooks :: control-plane-manager :: update_cluster_kubernetes_version", func() {

	var secretTemplate = `
---
apiVersion: v1
data:
  cluster-configuration.yaml: %s
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
type: Opaque
`

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context(fmt.Sprintf("Kubernetes version from secret is `%s`, should be changed to `Automatic`", config.DefaultKubernetesVersion), func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(fmt.Sprintf(secretTemplate, d8ClusterConfigurationSecretData(config.DefaultKubernetesVersion))))
			f.RunHook()
		})

		It("Hook should run, kubernetes version should change to desired", func() {
			Expect(f).To(ExecuteSuccessfully())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cluster-configuration")
			data := secret.Field("data.cluster-configuration\\.yaml")
			dataYaml, _ := base64.StdEncoding.DecodeString(data.String())
			expected := d8ClusterConfigurationSecretData("Automatic")
			expectedYaml, _ := base64.StdEncoding.DecodeString(expected)
			Expect(dataYaml).To(MatchYAML(expectedYaml))
		})
	})
	Context(fmt.Sprintf("Kubernetes version from secret is not `%s`, should not be changed", config.DefaultKubernetesVersion), func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(fmt.Sprintf(secretTemplate, d8ClusterConfigurationSecretData("1.100"))))
			f.RunHook()
		})

		It("Hook should run, kubernetes version should not change", func() {
			Expect(f).To(ExecuteSuccessfully())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cluster-configuration")
			data := secret.Field("data.cluster-configuration\\.yaml")
			dataYaml, _ := base64.StdEncoding.DecodeString(data.String())
			expected := d8ClusterConfigurationSecretData("1.100")
			expectedYaml, _ := base64.StdEncoding.DecodeString(expected)
			Expect(dataYaml).To(MatchYAML(expectedYaml))
		})
	})
})
