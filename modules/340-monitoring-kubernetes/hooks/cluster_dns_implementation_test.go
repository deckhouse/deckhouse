/*

User-stories:
1. There is deployment with labels k8s-app=kube-dns in kube-system namespace, hook must store its name to `global.discovery.clusterDNSImplementation`.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: monitoring-kubernetes :: hooks :: cluster_dns_implementation ::", func() {
	const (
		coreDnsDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    k8s-app: kube-dns
  name: coredns
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      k8s-app: kube-dns
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        k8s-app: kube-dns
    spec:
      containers:
      - image: coredns
        name: coredns
        resources: {}
`
		kubeDnsDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    k8s-app: kube-dns
  name: kube-dns
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      k8s-app: kube-dns
  strategy: {}
  template:
    metadata:
      labels:
        k8s-app: kube-dns
    spec:
      containers:
      - image: kube-dns
        name: kube-dns
        resources: {}
`
	)
	f := HookExecutionConfigInit(
		`{"monitoringKubernetes":{"internal":{}},"global":{"enabledModules":[]}}`,
		`{}`,
	)

	Context("Cluster with kube-dns", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kubeDnsDeployment))
			f.RunHook()
		})

		It("monitoringKubernetes.internal.clusterDNSImplementation must be 'kube-dns'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringKubernetes.internal.clusterDNSImplementation").String()).To(Equal("kube-dns"))
		})
	})

	Context("Cluster with coredns", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(coreDnsDeployment))
			f.RunHook()
		})

		It("monitoringKubernetes.internal.clusterDNSImplementation must be 'coredns'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringKubernetes.internal.clusterDNSImplementation").String()).To(Equal("coredns"))
		})
	})

	Context("KubeDNS module enabled with kube-dns deployment", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kubeDnsDeployment))
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["kube-dns"]`))
			f.RunHook()
		})

		It("monitoringKubernetes.internal.clusterDNSImplementation must be 'coredns'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringKubernetes.internal.clusterDNSImplementation").String()).To(Equal("coredns"))
		})
	})
})
