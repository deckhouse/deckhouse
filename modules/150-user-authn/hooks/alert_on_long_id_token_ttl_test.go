/*
Copyright 2026 Flant JSC

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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: alert on long idTokenTTL ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	expireOp := operation.MetricOperation{
		Group:  longIDTokenTTLMetricGroup,
		Action: operation.ActionExpireMetrics,
	}
	setOp := func(ttl string) operation.MetricOperation {
		return operation.MetricOperation{
			Name:   longIDTokenTTLMetricName,
			Value:  ptr.To(1.0),
			Labels: map[string]string{"id_token_ttl": ttl},
			Action: operation.ActionGaugeSet,
			Group:  longIDTokenTTLMetricGroup,
		}
	}

	Context("ModuleConfig without idTokenTTL", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  enabled: true
  version: 2
  settings: {}
`))
			f.RunHook()
		})

		It("Expires metric, sets nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(expireOp))
		})
	})

	Context("ModuleConfig with idTokenTTL below 6h", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  enabled: true
  version: 2
  settings:
    idTokenTTL: 5h59m
`))
			f.RunHook()
		})

		It("Expires metric, sets nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(expireOp))
		})
	})

	Context("ModuleConfig with idTokenTTL equal to 6h", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  enabled: true
  version: 2
  settings:
    idTokenTTL: 6h
`))
			f.RunHook()
		})

		It("emits the metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(expireOp, setOp("6h")))
		})
	})

	Context("ModuleConfig with idTokenTTL above 6h", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  enabled: true
  version: 2
  settings:
    idTokenTTL: 24h
`))
			f.RunHook()
		})

		It("emits the metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(expireOp, setOp("24h")))
		})
	})
})
