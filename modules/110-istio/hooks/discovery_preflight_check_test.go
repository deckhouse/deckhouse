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
	"context"
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const clusterConfigurationYaml = `---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
kubernetesVersion: "%s"
podSubnetCIDR: "10.111.0.0/16"
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: "10.222.0.0/16"
cloud:
  provider: OpenStack
`

var _ = Describe("Istio hooks :: discovery_preflight_check ::", func() {
	initValues := `
istio:
  internal:
    istioToK8sCompatibilityMap:
      "1.27": ["1.32", "1.33", "1.34", "1.35", "1.36"]
`
	f := HookExecutionConfigInit(initValues, "")

	Context("Cluster configuration secret with Automatic kubernetesVersion", func() {
		BeforeEach(func() {
			ccYaml := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(clusterConfigurationYaml, "Automatic")))
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  cluster-configuration.yaml: ` + ccYaml))
			f.RunHook()
		})

		It("Should publish compatibility map and automatic flag", func() {
			Expect(f).To(ExecuteSuccessfully())

			isAutomatic, exists := requirements.GetValue(isK8sVersionAutomaticKey)
			Expect(exists).To(BeTrue())
			Expect(isAutomatic).To(BeEquivalentTo(true))

			compatibilityMap, exists := requirements.GetValue(istioToK8sCompatibilityMapKey)
			Expect(exists).To(BeTrue())
			Expect(compatibilityMap).To(BeEquivalentTo(map[string][]string{
				"1.27": {"1.32", "1.33", "1.34", "1.35", "1.36"},
			}))
		})
	})

	Context("Cluster configuration secret with fixed kubernetesVersion", func() {
		BeforeEach(func() {
			ccYaml := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(clusterConfigurationYaml, "1.32")))
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  cluster-configuration.yaml: ` + ccYaml))
			f.RunHook()
		})

		It("Should not mark kubernetes version as automatic", func() {
			Expect(f).To(ExecuteSuccessfully())

			isAutomatic, exists := requirements.GetValue(isK8sVersionAutomaticKey)
			Expect(exists).To(BeTrue())
			Expect(isAutomatic).To(BeEquivalentTo(false))
		})
	})

	Context("Cluster configuration secret is read directly when snapshot is empty", func() {
		BeforeEach(func() {
			f.KubeStateSet("")

			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterConfigurationSecretName,
					Namespace: clusterConfigurationSecretNamespace,
				},
				Data: map[string][]byte{
					"cluster-configuration.yaml": []byte(fmt.Sprintf(clusterConfigurationYaml, "Automatic")),
				},
			}
			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				Secrets(clusterConfigurationSecretNamespace).
				Create(context.TODO(), secret, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			f.ValuesSet("global.discovery.kubernetesVersion", "1.32.5")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should detect Automatic from secret instead of global discovery", func() {
			Expect(f).To(ExecuteSuccessfully())

			isAutomatic, exists := requirements.GetValue(isK8sVersionAutomaticKey)
			Expect(exists).To(BeTrue())
			Expect(isAutomatic).To(BeEquivalentTo(true))
		})
	})

	Context("No cluster configuration secret", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.ValuesSet("global.discovery.kubernetesVersion", "1.32.5")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should fallback to global discovery kubernetes version", func() {
			Expect(f).To(ExecuteSuccessfully())

			isAutomatic, exists := requirements.GetValue(isK8sVersionAutomaticKey)
			Expect(exists).To(BeTrue())
			Expect(isAutomatic).To(BeEquivalentTo(false))
		})
	})
})
