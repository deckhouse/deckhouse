/*

User-stories:
1. There is coredns CM in cluster. It has `kubernetes my-cluster.xxx in-addr.arpa ip6.arpa` string with cluster domain. Hook must parse and store domain to `global.discovery.clusterDomain`.
2. There is kube-dns Pod in cluster. It has `--domain=my-cluster.xxx` arg with cluster domain. Hook must parse and store domain to `global.discovery.clusterDomain`.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery/cluster_domain ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateCM = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns
  namespace: kube-system
data:
  Corefile: |
    .:53 {
        errors
        health
        kubernetes mycluster.cm in-addr.arpa ip6.arpa {
           pods insecure
           upstream
           fallthrough in-addr.arpa ip6.arpa
           ttl 30
        }
        prometheus :9153
        forward . /etc/resolv.conf
        cache 30
        loop
        reload
        loadbalance
    }
`
		statePod = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    k8s-app: kube-dns
  name: kube-dns-111
  namespace: kube-system
spec:
  containers:
  - args:
    - asd
    - --domain=mycluster.pod.
    - qqq
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    k8s-app: kube-dns
  name: kube-dns-222
  namespace: kube-system
spec:
  containers:
  - args:
    - --domain=mycluster.pod.
    - qqq
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("global.discovery.clusterDomain must be 'cluster.local'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.clusterDomain").String()).To(Equal("cluster.local"))
		})

		Context("coredns CM created", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateCM))
				f.RunHook()
			})

			It("filterResult and `global.discovery.clusterDomain` must be 'mycluster.cm'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Get("0.filterResult").String()).To(Equal("mycluster.cm"))
				Expect(f.ValuesGet("global.discovery.clusterDomain").String()).To(Equal("mycluster.cm"))
			})
		})

		Context("kube-dns Pods created", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(statePod))
				f.RunHook()
			})

			It("filterResult and `global.discovery.clusterDomain` must be 'mycluster.pod'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Get("0.filterResult").String()).To(Equal("mycluster.pod"))
				Expect(f.ValuesGet("global.discovery.clusterDomain").String()).To(Equal("mycluster.pod"))
			})
		})
	})

	Context("Both coredns CM and kube-dns Pod are in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCM + statePod))
			f.RunHook()
		})

		It("`global.discovery.clusterDomain` must be 'mycluster.cm'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Get("0.objects.0.filterResult").String()).To(Equal("mycluster.cm"))
			Expect(f.BindingContexts.Get("1.objects.0.filterResult").String()).To(Equal("mycluster.pod"))
			Expect(f.ValuesGet("global.discovery.clusterDomain").String()).To(Equal("mycluster.cm"))
		})
	})
})
