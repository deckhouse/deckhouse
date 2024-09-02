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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	serviceYAML = `
---
apiVersion: v1
kind: Service
metadata:
  name: argocd-repo-server
  namespace: d8-delivery
  labels:
    service-helm-fix: true
spec:
  ports:
    - name: server
      port: 8081
      protocol: TCP
      targetPort: server
    - name: metrics
      port: 8084
      protocol: TCP
      targetPort: metrics`

	service2YAML = `
---
apiVersion: v1
kind: Service
metadata:
  name: argocd-server
  namespace: d8-delivery
spec:
  ports:
    - name: http
      port: 80
      protocol: TCP
      targetPort: server
    - name: https
      port: 443
      protocol: TCP
      targetPort: server
      targetPort: dns-tcp`
)

var _ = Describe("ArgoCD hooks :: migration_service_with_many_ports", func() {
	f := HookExecutionConfigInit("{}", "{}")

	Context("There is a broken service", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(serviceYAML + service2YAML))
			f.RunHook()
		})
		It("Service have been deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Service", "argocd-server").Exists()).To(BeFalse())
		})
	})
})
