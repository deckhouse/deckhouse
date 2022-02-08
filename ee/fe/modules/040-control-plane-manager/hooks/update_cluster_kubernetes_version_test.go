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
  provider: cloudProvider
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
	const (
		DeckhousePodIsReady = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: deckhouse
  name: deckhouse-pod
  namespace: d8-system
status:
  conditions:
  - status: "True"
    type: Ready
`
		DeckhousePodIsNotReady = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: deckhouse
  name: deckhouse-pod
  namespace: d8-system
status:
  conditions:
  - status: "False"
    type: Ready
`
	)

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

	Context("Kubernetes version from secret less than desired version, Deckhouse pod is not ready", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(DeckhousePodIsNotReady + fmt.Sprintf(secretTemplate, d8ClusterConfigurationSecretData("1.19"))))
			f.RunHook()
		})

		It("Hook should run, kubernetes version should not change to desired", func() {
			Expect(f).To(ExecuteSuccessfully())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cluster-configuration")
			data := secret.Field("data.cluster-configuration\\.yaml")
			Expect(data.Str).To(Equal(d8ClusterConfigurationSecretData("1.19")))
		})

	})
	Context("Kubernetes version from secret less than desired version, Deckhouse pod is ready", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(DeckhousePodIsReady + fmt.Sprintf(secretTemplate, d8ClusterConfigurationSecretData("1.19"))))
			f.RunHook()
		})

		It("Hook should run, kubernetes version should change to desired", func() {
			Expect(f).To(ExecuteSuccessfully())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cluster-configuration")
			data := secret.Field("data.cluster-configuration\\.yaml")
			Expect(data.Str).To(Equal(d8ClusterConfigurationSecretData(minimalKubernetesVersion)))
		})
	})
	Context("Kubernetes version from secret more or equal to desired version, Deckhouse pod is ready", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(DeckhousePodIsReady + fmt.Sprintf(secretTemplate, d8ClusterConfigurationSecretData("1.22"))))
			f.RunHook()
		})

		It("Hook should run, kubernetes version should not change to desired", func() {
			Expect(f).To(ExecuteSuccessfully())
			secret := f.KubernetesResource("Secret", "kube-system", "d8-cluster-configuration")
			data := secret.Field("data.cluster-configuration\\.yaml")
			Expect(data.Str).To(Equal(d8ClusterConfigurationSecretData("1.22")))
		})
	})
})
