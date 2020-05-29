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

var _ = Describe("Global hooks :: discovery/cluster_dns ::", func() {
	const (
		coreDnsDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
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
  creationTimestamp: null
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
      creationTimestamp: null
      labels:
        k8s-app: kube-dns
    spec:
      containers:
      - image: kube-dns
        name: kube-dns
        resources: {}
`
	)

	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)

	Context("Discover cluster dns", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(coreDnsDeployment))
			f.RunHook()
		})

		It("global.discovery.clusterDNSImplementation must be 'coredns'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.clusterDNSImplementation").String()).To(Equal("coredns"))
		})

		Context("Cluster dns changed to kube-dns", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(kubeDnsDeployment))
				f.RunHook()
			})

			It("global.discovery.clusterDNSImplementation must be 'kube-dns'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.clusterDNSImplementation").String()).To(Equal("kube-dns"))
			})
		})
	})
})
