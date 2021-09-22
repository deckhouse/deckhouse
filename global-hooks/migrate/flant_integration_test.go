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

var _ = Describe("Global hooks :: migrate/flant_integration ::", func() {
	const (
		initValuesString       = `{}`
		initConfigValuesString = `{}`
	)

	Context("Cluster with old modules", func() {
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
  prometheusMadisonIntegration: |
    madisonSelfSetupKey: "abcd"
    madisonAuthKey: "efgh"
  flantPricing: |
    kubeall:
      context: "test"
  flantPricingEnabled: "true"
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

		It("Hook migrates old values into new ones", func() {
			resCm, err := dependency.TestDC.K8sClient.CoreV1().
				ConfigMaps("d8-system").
				Get(context.TODO(), "deckhouse", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(resCm.Data["flantIntegration"]).To(MatchYAML(`
kubeall:
  context: "test"
madisonAuthKey: "efgh"
`))
			Expect(resCm.Data["flantPricing"]).ToNot(HaveLen(0))
			Expect(resCm.Data["prometheusMadisonIntegration"]).ToNot(HaveLen(0))
			Expect(resCm.Data["anotherModule"]).ToNot(HaveLen(0))
		})

		Context("next run", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.GenerateOnStartupContext())
				f.RunHook()
			})

			It("Hook does not fail", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Cleans up old values", func() {
				resCm, err := dependency.TestDC.K8sClient.CoreV1().
					ConfigMaps("d8-system").
					Get(context.TODO(), "deckhouse", metav1.GetOptions{})
				Expect(err).To(BeNil())
				Expect(resCm.Data["flantIntegration"]).To(MatchYAML(`
kubeall:
  context: "test"
madisonAuthKey: "efgh"
`))
				Expect(resCm.Data["flantPricing"]).To(HaveLen(0))
				Expect(resCm.Data["prometheusMadisonIntegration"]).To(HaveLen(0))
				Expect(resCm.Data["anotherModule"]).ToNot(HaveLen(0))
			})
		})
	})

	Context("With promscale settings", func() {
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
  prometheusMadisonIntegration: |
    madisonSelfSetupKey: "abcd"
    madisonAuthKey: "efgh"
  flantPricing: |
    kubeall:
      context: "test"
    promscale:
      url: https://me.tri.cs/write
  flantPricingEnabled: "true"
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

		It("Translates `promscale` to `metrics`", func() {
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
})
