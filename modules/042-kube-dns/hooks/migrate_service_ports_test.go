package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Kube DNS :: migrate_service_ports ::", func() {
	const (
		initValues = `
kubeDns:
  enableLogs: false
  internal:
    replicas: 2
    enablePodAntiAffinity: false
`
		initConfigValues = `{}`

		notMigratedDNSService = `
---
apiVersion: v1
kind: Service
metadata:
  name: d8-kube-dns
  namespace: kube-system
spec:
  ports:
  - name: dns
    port: 53
    protocol: UDP
    targetPort: 53
  - name: dns-tcp
    port: 53
    protocol: TCP
    targetPort: 53
`

		migratedDNSService = `
---
apiVersion: v1
kind: Service
metadata:
  name: d8-kube-dns
  namespace: kube-system
spec:
  ports:
  - name: dns
    port: 53
    protocol: UDP
    targetPort: 5353
  - name: dns-tcp
    port: 53
    protocol: TCP
    targetPort: 5353
`
	)

	f := HookExecutionConfigInit(initValues, initConfigValues)

	Context("Fresh cluster without a Service", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with a Service for migration", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(notMigratedDNSService))
			f.RunHook()
		})

		It("Hook must not fail, storage class should be present as a default", func() {
			Expect(f).To(ExecuteSuccessfully())
			service := f.KubernetesResource("Service", "kube-system", "d8-kube-dns")
			Expect(service.Exists()).To(BeTrue())
			Expect(service.Field("spec.ports").String()).To(MatchYAML(`
  - name: dns
    port: 53
    protocol: UDP
    targetPort: 5353
  - name: dns-tcp
    port: 53
    protocol: TCP
    targetPort: 5353
`))
		})
	})

	Context("Cluster with a migrated Service", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(migratedDNSService))
			f.RunHook()
		})

		It("Hook must not fail, storage class should be present as a default", func() {
			Expect(f).To(ExecuteSuccessfully())
			service := f.KubernetesResource("Service", "kube-system", "d8-kube-dns")
			Expect(service.Exists()).To(BeTrue())
			Expect(service.Field("spec.ports").String()).To(MatchYAML(`
  - name: dns
    port: 53
    protocol: UDP
    targetPort: 5353
  - name: dns-tcp
    port: 53
    protocol: TCP
    targetPort: 5353
`))
		})
	})

})
