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
var _ = Describe("Global hooks :: migrate/flant_integration_remove_kubeall_team ::", func() {
	const (
		initValuesString       = `{}`
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
			_, exists := resCm.Data["flantIntegration"]
			Expect(exists).To(BeFalse())
		})
	})

	Context("Without team", func() {
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
  flantIntegration: |
    kubeall:
      context: "test"
    madisonAuthKey: "efgh"
    metrics:
      url: https://me.tri.cs/write
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
			Expect(resCm.Data["flantIntegration"]).To(MatchYAML(`
kubeall:
  context: "test"
madisonAuthKey: "efgh"
metrics:
  url: https://me.tri.cs/write
`))
		})
	})

	Context("With team", func() {
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
  flantIntegration: |
    kubeall:
      team: "pravo"
      context: "test"
    madisonAuthKey: "efgh"
    metrics:
      url: https://me.tri.cs/write
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
			Expect(resCm.Data["flantIntegration"]).To(MatchYAML(`
kubeall:
  context: "test"
madisonAuthKey: "efgh"
metrics:
  url: https://me.tri.cs/write
`))
		})
	})

	Context("Only team", func() {
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
  flantIntegration: |
    kubeall:
      team: "pravo"
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

		It("Hook stores empty object when no fields left", func() {
			resCm, err := dependency.TestDC.K8sClient.CoreV1().
				ConfigMaps("d8-system").
				Get(context.TODO(), "deckhouse", metav1.GetOptions{})

			Expect(err).To(BeNil())
			Expect(resCm.Data["flantIntegration"]).To(MatchYAML(`kubeall: {}`))
		})
	})

	Context("No kubeall", func() {
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
  flantIntegration: |
    madisonAuthKey: "efgh"
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

		It("Hook does not change the config", func() {
			resCm, err := dependency.TestDC.K8sClient.CoreV1().
				ConfigMaps("d8-system").
				Get(context.TODO(), "deckhouse", metav1.GetOptions{})

			Expect(err).To(BeNil())
			Expect(resCm.Data["flantIntegration"]).To(MatchYAML(`madisonAuthKey: "efgh"`))
		})
	})
})
