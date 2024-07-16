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
	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Prometheus hooks :: deprecate trickster ingresses ::", func() {
	f := HookExecutionConfigInit(`{"prometheus":{"internal":{"grafana":{}}}}`, ``)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should not expose deprecation metrics", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(0))
		})

		Context("After adding trickster ingress", func() {
			BeforeEach(func() {
				err := createNs(tricksterIngressNamespace)
				Expect(err).To(BeNil())

				err = createIngress(tricksterIngress, tricksterIngressNamespace)
				Expect(err).To(BeNil())
				f.RunHook()
			})

			It("Should start exposing metrics about deprecation", func() {
				Expect(f).To(ExecuteSuccessfully())
				m := f.MetricsCollector.CollectedMetrics()
				Expect(m).To(HaveLen(1))
				Expect(m[0].Name).To(Equal("d8_trickster_deprecated_ingresses"))
				Expect(m[0].Labels).To(Equal(map[string]string{
					"ingress":   "trickster",
					"namespace": "trickster_namespace",
					"backend":   "trickster",
				}))
			})

		})
	})

})

const tricksterIngressNamespace = "trickster-namespace"

const tricksterIngress = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: trickster
spec:
  ingressClassName: nginx
  rules:
  - host: trickster.host
    http:
      paths:
      - backend:
          service:
            name: trickster
            port:
              name: https
        path: /trickster(/|$)(.*)
        pathType: ImplementationSpecific
`

func createNs(namespace string) error {
	var ns corev1.Namespace
	_ = yaml.Unmarshal([]byte(namespace), &ns)

	_, err := dependency.TestDC.MustGetK8sClient().
		CoreV1().
		Namespaces().
		Create(context.TODO(), &ns, metav1.CreateOptions{})
	return err
}

func createIngress(ingress, namespace string) error {
	var i netv1.Ingress
	_ = yaml.Unmarshal([]byte(ingress), &i)

	_, err := dependency.TestDC.MustGetK8sClient().
		NetworkingV1().
		Ingresses(namespace).
		Create(context.TODO(), &i, metav1.CreateOptions{})
	return err
}
