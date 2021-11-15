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

/*

User-stories:
1. There is deployment with labels k8s-app=kube-dns or k8s-app=coredns in kube-system namespace, hook must store its name to `global.discovery.clusterDNSImplementation`.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: monitoring-kubernetes :: hooks :: cluster_dns_implementation ::", func() {
	const (
		coreDNSDeployment = `
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
		kubeDNSDeployment = `
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

		kubeDNSMCSDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    k8s-app: coredns
  name: coredns
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      k8s-app: coredns
  strategy: {}
  template:
    metadata:
      labels:
        k8s-app: coredns
    spec:
      containers:
      - image: coredns
        name: coredns
        resources: {}
`
	)
	f := HookExecutionConfigInit(
		`{"monitoringKubernetes":{"internal":{}},"global":{"enabledModules":[]}}`,
		`{}`,
	)

	Context("Cluster with kube-dns", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kubeDNSDeployment))
			f.RunHook()
		})

		It("monitoringKubernetes.internal.clusterDNSImplementation must be 'kube-dns'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringKubernetes.internal.clusterDNSImplementation").String()).To(Equal("kube-dns"))
		})
	})

	Context("Cluster with coredns", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(coreDNSDeployment))
			f.RunHook()
		})

		It("monitoringKubernetes.internal.clusterDNSImplementation must be 'coredns'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringKubernetes.internal.clusterDNSImplementation").String()).To(Equal("coredns"))
		})
	})

	Context("KubeDNS module enabled with kube-dns deployment", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kubeDNSDeployment))
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["kube-dns"]`))
			f.RunHook()
		})

		It("monitoringKubernetes.internal.clusterDNSImplementation must be 'coredns'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringKubernetes.internal.clusterDNSImplementation").String()).To(Equal("coredns"))
		})
	})

	Context("KubeDNS module disabled. Managed MCS cluster with stock deployment", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kubeDNSMCSDeployment))
			f.RunHook()
		})

		It("monitoringKubernetes.internal.clusterDNSImplementation must be 'coredns'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringKubernetes.internal.clusterDNSImplementation").String()).To(Equal("coredns"))
		})
	})
})
