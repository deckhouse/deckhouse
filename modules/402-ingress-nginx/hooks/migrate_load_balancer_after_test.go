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

var _ = Describe("ingress-nginx :: hooks :: migrate_load_balancer_after ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"defaultControllerVersion": 0.25, "internal": {"webhookCertificates":{}}}}`, "")

	dsControllerMainYAML := `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: controller-main
  namespace: d8-ingress-nginx
  labels:
    name: main
    app: controller
  annotations:
    ingress-nginx-controller.deckhouse.io/inlet: LoadBalancer
status:
  desiredNumberScheduled: 2
`
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
  annotations:
    ingress-nginx-controller.deckhouse.io/inlet: LoadBalancer
status:
  readyReplicas: 2
  replicas: 2
`
	dNotReadyControllerMainYAML := `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-main
  namespace: d8-ingress-nginx
  labels:
    name: main
    app: controller
  annotations:
    ingress-nginx-controller.deckhouse.io/inlet: LoadBalancer
status:
  readyReplicas: 1
  replicas: 2
`
	dsControllerOtherYAML := `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: controller-other
  namespace: d8-ingress-nginx
  labels:
    name: main
    app: controller
  annotations:
    ingress-nginx-controller.deckhouse.io/inlet: HostPort
`

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())

			f.RunHook()
		})

		It("must be execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with ingress controller DaemonSet and ready Deployment", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(dsControllerMainYAML + dControllerMainYAML))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())

			f.RunHook()
		})

		It("must be execute successfully, and delete DaemonSet", func() {
			Expect(f).To(ExecuteSuccessfully())

			dsControllerMain := f.KubernetesResource("DaemonSet", namespace, "controller-main")
			Expect(dsControllerMain.Exists()).To(BeFalse())
		})
	})

	Context("Cluster with ingress controller DaemonSet and not ready Deployment", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(dsControllerMainYAML + dNotReadyControllerMainYAML))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())

			f.RunHook()
		})

		It("must be execute successfully, and keep DaemonSet", func() {
			Expect(f).To(ExecuteSuccessfully())

			dsControllerMain := f.KubernetesResource("DaemonSet", namespace, "controller-main")
			Expect(dsControllerMain.Exists()).To(BeTrue())
		})
	})

	Context("Cluster with ingress controller DaemonSet, not suitable for migration", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(dsControllerOtherYAML))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())

			f.RunHook()
		})

		It("must be execute successfully, set replicas and chang helm release secret", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

})
