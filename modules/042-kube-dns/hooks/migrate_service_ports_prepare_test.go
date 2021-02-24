package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Kube DNS :: migrate_service_ports_prepare ::", func() {
	const (
		initValues = `
kubeDns:
  enableLogs: false
  internal:
    replicas: 2
    enablePodAntiAffinity: false
`
		initConfigValues = `{}`

		originalState = `
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

		migratingState = `
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
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    k8s-app: kube-dns
  name: d8-kube-dns-c48f54f56-bf8zx
  namespace: kube-system
spec:
  containers:
    - name: coredns
      ports:
        - containerPort: 53
          name: dns
          protocol: UDP
        - containerPort: 53
          name: tcp
          protocol: TCP
  nodeName: main-master-0
status:
  conditions:
    - status: "True"
      type: Ready
`

		migratedState = `
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
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    k8s-app: kube-dns
  name: d8-kube-dns-c48f54f56-bf8zx
  namespace: kube-system
spec:
  containers:
    - name: coredns
      ports:
        - containerPort: 53
          name: dns-old
          protocol: UDP
        - containerPort: 53
          name: tcp-old
          protocol: TCP
        - containerPort: 5353
          name: dns
          protocol: UDP
        - containerPort: 5353
          name: tcp
          protocol: TCP
  nodeName: main-master-0
status:
  conditions:
    - status: "True"
      type: Ready
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
			f.BindingContexts.Set(f.KubeStateSet(originalState))
			f.RunHook()
		})

		It("Hook must not fail, migration flag should be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("kubeDns.internal.migration").Bool()).To(Equal(true))
		})
	})

	Context("Cluster with a Service for migration, migration flag set, and pods with old ports", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(migratingState))
			f.ValuesSet("kubeDns.internal.migration", true)
			f.RunHook()
		})

		It("Hook must not fail, migration flag should be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("kubeDns.internal.migration").Bool()).To(Equal(true))
		})
	})

	Context("Cluster with a Service for migration, migration flag set, and pods with old and new ports", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(migratedState))
			f.ValuesSet("kubeDns.internal.migration", true)
			f.RunHook()
		})

		It("Hook must not fail, migration flag should be unset", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("kubeDns.internal.migration").Bool()).To(Equal(false))
		})
	})

})
