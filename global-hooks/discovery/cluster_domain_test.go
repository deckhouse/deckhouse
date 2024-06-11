// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*

User-stories:
1. There is coredns CM in cluster. It has `kubernetes my-cluster.xxx in-addr.arpa ip6.arpa` string with cluster domain. Hook must parse and store domain to `global.discovery.clusterDomain`.
2. There is kube-dns Pod in cluster. It has `--domain=my-cluster.xxx` arg with cluster domain. Hook must parse and store domain to `global.discovery.clusterDomain`.
3. The global cluster Configuration variables have a value for clusterDomain, which we use to define `global.discovery.clusterDomain`

*/

package hooks

import (
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: cluster_domain ::", func() {
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

	Context("Cluster with pods and a domain in discovery", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(statePod))
			f.ValuesSet("global.discovery.clusterDomain", "test-cluster.local")
			f.RunHook()
		})

		It("global.discovery.clusterDomain must be 'test-cluster.local'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.clusterDomain").String()).To(Equal("test-cluster.local"))
		})
	})

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("global.discovery.clusterDomain must be 'cluster.local'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.clusterDomain").String()).To(Equal("cluster.local"))
		})
	})

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
		})

		Context("coredns CM created", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateCM))
				f.RunHook()
			})

			It("filterResult and `global.discovery.clusterDomain` must be 'mycluster.cm'", func() {
				Expect(f).To(ExecuteSuccessfully())
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
			Expect(f.ValuesGet("global.discovery.clusterDomain").String()).To(Equal("mycluster.cm"))
		})
	})

	Context("Cluster with clusterDomain in clusterConfiguration", func() {
		f := HookExecutionConfigInit(`{"global": {"discovery": {"clusterDomain": "test.local"}}}`, initConfigValuesString)

		BeforeEach(func() {
			var (
				stateAClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
cloud:
  provider: OpenStack
  prefix: kube
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.29"
clusterDomain: "test.local"
`
				stateA = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration))
			)

			f.BindingContexts.Set(f.KubeStateSet(stateA + statePod))
			f.RunHook()
		})

		It("`global.discovery.clusterDomain` must be not set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.clusterDomain").String()).
				To(Equal("test.local"))
		})
	})

})
