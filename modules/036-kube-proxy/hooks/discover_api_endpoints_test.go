/*
Copyright 2021 Flant JSC

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

var _ = Describe("Modules :: kube-proxy :: hooks :: discover_apiserver_endpoints ::", func() {
	const (
		stateSingleEndpointSliceWithOneIPAndOnePort = `
apiVersion: discovery.k8s.io/v1
kind: EndpointSlice
metadata:
  name: kubernetes-one-one
  namespace: default
  labels:
    kubernetes.io/service-name: kubernetes
endpoints:
- addresses:
  - 10.0.1.192
ports:
- name: https
  port: 6443
  protocol: TCP
`

		stateSingleEndpointSliceWithThreeIPAndOnePort = `
apiVersion: discovery.k8s.io/v1
kind: EndpointSlice
metadata:
  name: kubernetes-three-one
  namespace: default
  labels:
    kubernetes.io/service-name: kubernetes
endpoints:
- addresses:
  - 10.0.1.192
- addresses:
  - 10.0.1.193
- addresses:
  - 10.0.1.194
ports:
- name: https
  port: 6443
  protocol: TCP
`
		stateSingleEndpointSliceWithTwoIPAndTwoPorts = `
apiVersion: discovery.k8s.io/v1
kind: EndpointSlice
metadata:
  name: kubernetes-two-two
  namespace: default
  labels:
    kubernetes.io/service-name: kubernetes
endpoints:
- addresses:
  - 10.0.1.192
- addresses:
  - 10.0.1.193
ports:
- name: https
  port: 6443
  protocol: TCP
- name: https-whatever
  port: 8443
  protocol: TCP
`
		stateThreeEndpointSlicesWithDublicatedAndMultipleIPPorts = `
---
apiVersion: discovery.k8s.io/v1
kind: EndpointSlice
metadata:
  name: kubernetes-slice-one
  namespace: default
  labels:
    kubernetes.io/service-name: kubernetes
endpoints:
- addresses:
  - 10.0.1.192
  - 10.0.1.192
- addresses:
  - 10.0.1.192
ports:
- name: https
  port: 6443
  protocol: TCP
---
apiVersion: discovery.k8s.io/v1
kind: EndpointSlice
metadata:
  name: kubernetes-slice-two
  namespace: default
  labels:
    kubernetes.io/service-name: kubernetes
endpoints:
- addresses:
  - 10.0.1.193
- addresses:
  - 10.0.1.194
ports:
- name: https
  port: 8443
  protocol: TCP
---
apiVersion: discovery.k8s.io/v1
kind: EndpointSlice
metadata:
  name: kubernetes-slice-three
  namespace: default
  labels:
    kubernetes.io/service-name: kubernetes
endpoints:
- addresses:
  - 10.0.1.195
ports:
- name: https
  port: 6443
  protocol: TCP
- name: https-whatever
  port: 8443
  protocol: TCP
`
	)

	f := HookExecutionConfigInit(`{"kubeProxy":{"internal": {}}}`, `{}`)

	Context("Single EndpointSlice resource with one IP and one port", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateSingleEndpointSliceWithOneIPAndOnePort))
			f.RunHook()
		})

		It("`kubeProxy.internal.clusterMasterAddresses` must be ['10.0.1.192:6443']", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("kubeProxy.internal.clusterMasterAddresses").String()).To(MatchJSON(`["10.0.1.192:6443"]`))
		})
	})

	Context("Single EndpointSlice resource with three IP and one port", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateSingleEndpointSliceWithThreeIPAndOnePort))
			f.RunHook()
		})

		It("`kubeProxy.internal.clusterMasterAddresses` must be ['10.0.1.192:6443','10.0.1.193:6443','10.0.1.194:6443']", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("kubeProxy.internal.clusterMasterAddresses").String()).To(MatchJSON(`["10.0.1.192:6443","10.0.1.193:6443","10.0.1.194:6443"]`))
		})
	})

	Context("Single EndpointSlice resource two IP and two ports", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateSingleEndpointSliceWithTwoIPAndTwoPorts))
			f.RunHook()
		})

		It("`kubeProxy.internal.clusterMasterAddresses` must be ['10.0.1.192:6443','10.0.1.192:8443','10.0.1.193:6443','10.0.1.193:8443']", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("kubeProxy.internal.clusterMasterAddresses").String()).To(MatchJSON(`["10.0.1.192:6443","10.0.1.192:8443","10.0.1.193:6443","10.0.1.193:8443"]`))
		})
	})

	Context("Three EndpointSlice resources with mixed combination and dublicated IP", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateThreeEndpointSlicesWithDublicatedAndMultipleIPPorts))
			f.RunHook()
		})

		It("`kubeProxy.internal.clusterMasterAddresses` must be ['10.0.1.192:6443','10.0.1.193:6443','10.0.1.193:8443','10.0.1.195:6443','10.0.1.195:8443']", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("kubeProxy.internal.clusterMasterAddresses").String()).To(MatchJSON(`["10.0.1.192:6443","10.0.1.193:8443","10.0.1.194:8443","10.0.1.195:6443","10.0.1.195:8443"]`))
		})
	})
})
