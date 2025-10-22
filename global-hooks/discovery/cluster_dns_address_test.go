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

package hooks

/*

User-stories:
1. There is Service kube-system/kube-dns with clusterIP, hook must store it to `global.discovery.clusterDNSAddress`.

*/

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString       = `{"global": {"discovery": {}}}`
	initConfigValuesString = `{}`
)

var _ = Describe("Global hooks :: discovery :: cluster_dns_address ::", func() {
	const (
		stateA = `
---
apiVersion: v1
kind: Service
metadata:
  name: d8-kube-dns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
spec:
  clusterIP: 192.168.0.10
`
		stateB = `
---
apiVersion: v1
kind: Service
metadata:
  name: d8-kube-dns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
spec:
  clusterIP: 192.168.0.42
`
		stateC = `
---
apiVersion: v1
kind: Service
metadata:
  name: kube-dns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
spec:
  type: ExternalName
  externalName: d8-kube-dns.kube-system.svc.cluster.local
`
		stateD = `
---
apiVersion: v1
kind: Service
metadata:
  name: kube-dns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
spec:
  clusterIP: 192.168.0.10
`
		stateE = `
---
apiVersion: v1
kind: Service
metadata:
  name: kube-dns-upstream
  namespace: kube-system
  labels:
    k8s-app: kube-dns
spec:
  clusterIP: 192.168.0.42
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Cluster started with clusterIP = '192.168.0.10'", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateA))
			f.RunHook()
		})

		It("global.discovery.clusterDNSAddress must be '192.168.0.10'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.clusterDNSAddress").String()).To(Equal("192.168.0.10"))
		})

		Context("clusterIP changed to 192.168.0.42", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateB))
				f.RunHook()
			})

			It("global.discovery.clusterDNSAddress must be '192.168.0.42'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.clusterDNSAddress").String()).To(Equal("192.168.0.42"))
			})

			Context("Adding CNAME service without clusterIP", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateB + stateC))
					f.RunHook()
				})

				It("global.discovery.clusterDNSAddress must be '192.168.0.42'", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("global.discovery.clusterDNSAddress").String()).To(Equal("192.168.0.42"))
				})
			})
		})
	})

	Context("Fresh cluster without dns service", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("should fail", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
		})
	})

	Context("Cluster started with kube-dns service name and clusterIP = '192.168.0.10'", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateD))
			f.RunHook()
		})

		It("global.discovery.clusterDNSAddress must be '192.168.0.10'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.clusterDNSAddress").String()).To(Equal("192.168.0.10"))
		})

		Context("Adding another clusterIP with same labels", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateE))
				f.RunHook()
			})

			It("global.discovery.clusterDNSAddress must be same as previous'192.168.0.10'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.clusterDNSAddress").String()).To(Equal("192.168.0.42"))
			})
		})
	})
})
