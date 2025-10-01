/*
Copyright 2025 Flant JSC

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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/modules/402-ingress-nginx/hooks/internal"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("ingress-nginx :: hooks :: setAnnotationValidationSuspendedHandleIngressNginxControllers ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"internal":{"ingressControllers":[]}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", false)
	f.RegisterCRD(internal.IngressNginxControllerGVR.Group, internal.IngressNginxControllerGVR.Version, "IngressNginxController", false)

	Context("No controllers, no ConfigMap", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("does nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("ingressNginx.internal.ingressControllers").Array()).To(BeEmpty())
			Expect(hasMetric(f.MetricsCollector.CollectedMetrics())).To(BeFalse())
		})
	})

	Context("Less than 5 controllers, no ConfigMap", func() {
		BeforeEach(func() {
			f.KubeStateSet(threeControllers)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("does nothing and does not set metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("ingressNginx.internal.ingressControllers").Array()).To(BeEmpty())
			Expect(hasMetric(f.MetricsCollector.CollectedMetrics())).To(BeFalse())
		})
	})

	Context("5 controllers, no ConfigMap", func() {
		BeforeEach(func() {
			f.KubeStateSet(fiveControllers)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("adds suspended annotation via patch and sets metric", func() {
			Expect(f).To(ExecuteSuccessfully())

			incList, err := f.KubeClient().Dynamic().Resource(internal.IngressNginxControllerGVR).List(context.Background(), metav1.ListOptions{})
			Expect(err).To(BeNil())
			Expect(len(incList.Items)).To(Equal(5))

			for _, item := range incList.Items {
				annotations := item.GetAnnotations()
				_, has := annotations[internal.IngressNginxControllerSuspendAnnotation]
				Expect(has).To(BeTrue(), "controller %s is missing the suspended annotation", item.GetName())
			}

			Expect(hasMetric(f.MetricsCollector.CollectedMetrics())).To(BeTrue())
		})
	})

	Context("5 controllers, ConfigMap exists", func() {
		BeforeEach(func() {
			f.KubeStateSet(fiveControllers + "\n---\n" + configMapSuspended)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("does nothing because ConfigMap exists", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("ingressNginx.internal.ingressControllers").Array()).To(BeEmpty())
			Expect(hasMetric(f.MetricsCollector.CollectedMetrics())).To(BeFalse())
		})
	})

	Context("Metric expires when annotations are removed", func() {
		BeforeEach(func() {
			// Start with 5 controllers, some annotated
			f.KubeStateSet(fiveControllersWithAnnotation)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()

			// Remove annotations from all controllers and add ConfigMap
			f.KubeStateSet(fiveControllers + "\n---\n" + configMapSuspended)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("expires the validation suspended metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(hasMetric(f.MetricsCollector.CollectedMetrics())).To(BeFalse())
		})
	})
})

func hasMetric(metrics []operation.MetricOperation) bool {
	const metricName = "ingress_nginx_validation_suspended"
	for _, m := range metrics {
		if m.Name == metricName {
			return true
		}
	}
	return false
}

const (
	configMapSuspended = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: ingress-nginx-validation-suspended
  namespace: d8-ingress-nginx
`

	threeControllers = `
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: ctrl-1
spec:
  validationEnabled: true
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: ctrl-2
spec:
  validationEnabled: true
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: ctrl-3
spec:
  validationEnabled: true
`

	fiveControllers = `
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: ctrl-1
spec:
  validationEnabled: true
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: ctrl-2
spec:
  validationEnabled: true
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: ctrl-3
spec:
  validationEnabled: true
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: ctrl-4
spec:
  validationEnabled: true
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: ctrl-5
spec:
  validationEnabled: true
`
)

const fiveControllersWithAnnotation = `
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: ctrl-1
  annotations:
    network.deckhouse.io/ingress-nginx-validation-suspended: ""
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: ctrl-2
  annotations:
    network.deckhouse.io/ingress-nginx-validation-suspended: ""
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: ctrl-3
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: ctrl-4
  annotations:
    network.deckhouse.io/ingress-nginx-validation-suspended: ""
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: ctrl-5
`
