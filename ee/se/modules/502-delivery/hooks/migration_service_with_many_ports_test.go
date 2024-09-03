/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
    migration.deckhouse.io/fix-services-broken-by-helm: done
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
