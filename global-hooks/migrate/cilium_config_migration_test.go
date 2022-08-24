// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// THIS IS AN EXAMPLE MIGRATION TEST SUITE, NOT USEFUL BY ITSELF
var _ = Describe("Global hooks :: migrate/cilium_config ::", func() {
	const (
		initValuesString                  = `{}`
		initValuesStringWithCloudProvider = `
{
  "global": {
    "clusterConfiguration": {
      "apiVersion": "deckhouse.io/v1",
      "cloud": {
        "prefix": "dev",
        "provider": "OpenStack"
      },
      "clusterDomain": "cluster.local",
      "clusterType": "Cloud",
      "defaultCRI": "Containerd",
      "kind": "ClusterConfiguration",
      "kubernetesVersion": "1.20",
      "podSubnetCIDR": "10.111.0.0/16",
      "podSubnetNodeCIDRPrefix": "24",
      "serviceSubnetCIDR": "10.222.0.0/16",
    }
  }
}`
		initConfigValuesString = `{}`
	)

	Context("No config", func() {
		const (
			cmDeckhouse = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: deckhouse
  namespace: d8-system
data:
  anotherModule: |
    yes: no
`
		)

		var cm v1.ConfigMap
		_ = yaml.Unmarshal([]byte(cmDeckhouse), &cm)

		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {
			f.KubeStateSet("")

			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				ConfigMaps("d8-system").
				Create(context.TODO(), &cm, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook does not add settings", func() {
			resCm, err := dependency.TestDC.K8sClient.CoreV1().
				ConfigMaps("d8-system").
				Get(context.TODO(), "deckhouse", metav1.GetOptions{})
			Expect(err).To(BeNil())
			_, exists := resCm.Data["cniCilium"]
			Expect(exists).To(BeFalse())
		})
	})

	Context("With tunnelMode VXLAN", func() {
		const (
			cmDeckhouse = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: deckhouse
  namespace: d8-system
data:
  anotherModule: |
    yes: no
  cniCilium: |
    tunnelMode: VXLAN
`
		)

		var cm v1.ConfigMap
		_ = yaml.Unmarshal([]byte(cmDeckhouse), &cm)

		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {
			f.KubeStateSet("")

			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				ConfigMaps("d8-system").
				Create(context.TODO(), &cm, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook does set type = VXLAN", func() {
			resCm, err := dependency.TestDC.K8sClient.CoreV1().
				ConfigMaps("d8-system").
				Get(context.TODO(), "deckhouse", metav1.GetOptions{})

			Expect(err).To(BeNil())
			Expect(resCm.Data["cniCilium"]).To(MatchYAML(`
mode: VXLAN
`))
		})
	})

	Context("No config, but with cloud provider, which needs direct node routes mode", func() {
		const (
			cmDeckhouse = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: deckhouse
  namespace: d8-system
data:
  anotherModule: |
    yes: no
`
		)

		var cm v1.ConfigMap
		_ = yaml.Unmarshal([]byte(cmDeckhouse), &cm)

		f := HookExecutionConfigInit(initValuesStringWithCloudProvider, initConfigValuesString)

		BeforeEach(func() {
			f.KubeStateSet("")

			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				ConfigMaps("d8-system").
				Create(context.TODO(), &cm, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should add settings", func() {
			resCm, err := dependency.TestDC.K8sClient.CoreV1().
				ConfigMaps("d8-system").
				Get(context.TODO(), "deckhouse", metav1.GetOptions{})

			Expect(err).To(BeNil())
			Expect(resCm.Data["cniCilium"]).To(MatchYAML(`
mode: DirectWithNodeRoutes
`))
		})
	})

	Context("With cloud provider, which needs direct node routes mode, but tunnelMode is VXLAN", func() {
		const (
			cmDeckhouse = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: deckhouse
  namespace: d8-system
data:
  cniCilium: |
    tunnelMode: VXLAN
  anotherModule: |
    yes: no
`
		)

		var cm v1.ConfigMap
		_ = yaml.Unmarshal([]byte(cmDeckhouse), &cm)

		f := HookExecutionConfigInit(initValuesStringWithCloudProvider, initConfigValuesString)

		BeforeEach(func() {
			f.KubeStateSet("")

			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				ConfigMaps("d8-system").
				Create(context.TODO(), &cm, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should add settings", func() {
			resCm, err := dependency.TestDC.K8sClient.CoreV1().
				ConfigMaps("d8-system").
				Get(context.TODO(), "deckhouse", metav1.GetOptions{})

			Expect(err).To(BeNil())
			Expect(resCm.Data["cniCilium"]).To(MatchYAML(`
mode: VXLAN
`))
		})
	})

	Context("With cloud provider, which needs direct node routes mode, but createNodeRoutes is set to false", func() {
		const (
			cmDeckhouse = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: deckhouse
  namespace: d8-system
data:
  cniCilium: |
    createNodeRoutes: false
  anotherModule: |
    yes: no
`
		)

		var cm v1.ConfigMap
		_ = yaml.Unmarshal([]byte(cmDeckhouse), &cm)

		f := HookExecutionConfigInit(initValuesStringWithCloudProvider, initConfigValuesString)

		BeforeEach(func() {
			f.KubeStateSet("")

			_, err := dependency.TestDC.MustGetK8sClient().
				CoreV1().
				ConfigMaps("d8-system").
				Create(context.TODO(), &cm, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook should add settings", func() {
			resCm, err := dependency.TestDC.K8sClient.CoreV1().
				ConfigMaps("d8-system").
				Get(context.TODO(), "deckhouse", metav1.GetOptions{})

			Expect(err).To(BeNil())
			_, exists := resCm.Data["cniCilium"]
			Expect(exists).To(BeFalse())
		})
	})
})
