/*
Copyright 2021 Flant CJSC

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

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("ingress-nginx :: hooks :: migrate_load_balancer_before ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"defaultControllerVersion": 0.25, "internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", false)

	dControllerMainYAML := `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-main
  namespace: d8-ingress-nginx
  labels:
    name: main
    app: controller
    app.kubernetes.io/managed-by: Helm
  annotations:
    ingress-nginx-controller.deckhouse.io/inlet: LoadBalancer
    meta.helm.sh/release-name: foo
    meta.helm.sh/release-namespace: d8-ingress-nginx
status:
  replicas: 6
`
	ingressControllerMainYAML := `
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
`
	dControllerOtherYAML := `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-other
  namespace: d8-ingress-nginx
  annotations:
    ingress-nginx-controller.deckhouse.io/inlet: LoadBalancer
    meta.helm.sh/release-name: foo
    meta.helm.sh/release-namespace: d8-ingress-nginx
    ingress-nginx-controller.deckhouse.io/inlet: HostPort
  labels:
    name: main
    app: controller
    app.kubernetes.io/managed-by: Helm
`
	Context("Cluster with ingress controller and its Deployment", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(dControllerMainYAML + ingressControllerMainYAML))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())

			f.RunHook()
		})

		It("must be execute successfully, set replicas and chang helm release secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			ingressControllerMain := f.KubernetesResource("IngressNginxController", "", "main")
			Expect(ingressControllerMain.Exists()).To(BeTrue())
			Expect(ingressControllerMain.Field("spec.minReplicas").Int()).To(Equal(int64(3)))
			Expect(ingressControllerMain.Field("spec.maxReplicas").Int()).To(Equal(int64(3)))

			dep := f.KubernetesResource("Deployment", "d8-ingress-nginx", "controller-main")
			Expect(dep.Exists()).To(BeTrue())
			Expect(dep.Field("metadata.annotations.meta\\.helm\\.sh/release-name").Exists()).To(BeFalse())
			Expect(dep.Field("metadata.annotations.meta\\.helm\\.sh/release-namespace").Exists()).To(BeFalse())
			Expect(dep.Field("metadata.labels.app\\.kubernetes\\.io/managed-by").Exists()).To(BeFalse())

			Expect(dep.Field("metadata.annotations.helm\\.sh/resource-policy").String()).To(BeEquivalentTo("keep"))
		})
	})

	Context("Cluster with ingress controller DaemonSet, not suitable for migration", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(dControllerOtherYAML))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())

			f.RunHook()
		})

		It("must be execute successfully, set replicas and chang helm release secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			dep := f.KubernetesResource("Deployment", "d8-ingress-nginx", "controller-other")
			Expect(dep.Exists()).To(BeTrue())

			Expect(dep.Field("metadata.labels.app\\.kubernetes\\.io/managed-by").Exists()).To(BeTrue())
			Expect(dep.Field("metadata.annotations.helm\\.sh/resource-policy").Exists()).To(BeFalse())
		})
	})

})
