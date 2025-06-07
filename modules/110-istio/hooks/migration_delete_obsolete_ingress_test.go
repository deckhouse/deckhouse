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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("istio :: hooks :: delete_obsolete_ingress ::", func() {
	const (
		d8IstioNamespace = `
---
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    meta.helm.sh/release-name: istio
    meta.helm.sh/release-namespace: d8-system
  labels:
    app.kubernetes.io/managed-by: Helm
    extended-monitoring.deckhouse.io/enabled: ""
    heritage: deckhouse
    kubernetes.io/metadata.name: d8-istio
    module: istio
    prometheus.deckhouse.io/rules-watcher-enabled: "true"
  name: d8-istio
spec:
  finalizers:
  - kubernetes
`
		kialiRewriteIngress = `
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
  name: kiali-rewrite
  namespace: d8-istio
spec:
  ingressClassName: nginx
  rules:
  - host: istio.example.com
    http:
      paths:
      - backend:
          service:
            name: kiali
            port:
              name: http
        path: /
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - istio.example.com
    secretName: istio-ingress-tls
`
		kialiIngress = `
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
  name: kiali
  namespace: d8-istio
spec:
  ingressClassName: nginx
  rules:
  - host: istio.example.com
    http:
      paths:
      - backend:
          service:
            name: kiali
            port:
              name: http
        path: /
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - istio.example.com
    secretName: istio-ingress-tls
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("An empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.KubeStateSet(``)
			f.RunGoHook()
		})

		It("Hook is executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with obsolete ingress", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.KubeStateSet(``)

			var (
				ns       *corev1.Namespace
				ingress1 *netv1.Ingress
				ingress2 *netv1.Ingress
			)

			_ = yaml.Unmarshal([]byte(d8IstioNamespace), &ns)
			_ = yaml.Unmarshal([]byte(kialiRewriteIngress), &ingress1)
			_ = yaml.Unmarshal([]byte(kialiIngress), &ingress2)

			k8sClient := f.BindingContextController.FakeCluster().Client
			_, err := k8sClient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			_, err = k8sClient.NetworkingV1().Ingresses(istioNs).Create(context.TODO(), ingress1, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			_, err = k8sClient.NetworkingV1().Ingresses(istioNs).Create(context.TODO(), ingress2, metav1.CreateOptions{})
			Expect(err).To(BeNil())
			f.RunGoHook()
		})

		It("Check that the obsolete ingress was deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			k8sClient := f.BindingContextController.FakeCluster().Client
			_, err := k8sClient.CoreV1().Namespaces().Get(context.Background(), istioNs, metav1.GetOptions{})
			Expect(err).To(BeNil())
			_, err = k8sClient.NetworkingV1().Ingresses(istioNs).Get(context.Background(), obsoleteIngress, metav1.GetOptions{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
			_, err = k8sClient.NetworkingV1().Ingresses(istioNs).Get(context.Background(), "kiali", metav1.GetOptions{})
			Expect(err).To(BeNil())
		})
	})
})
