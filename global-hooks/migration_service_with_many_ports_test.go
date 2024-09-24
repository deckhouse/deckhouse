/*
Copyright 2024 Flant JSC

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

const (
	serviceYAML = `
---
apiVersion: v1
kind: Service
metadata:
  name: d8-kube-dns
  namespace: kube-system
spec:
  clusterIP: 10.222.0.10
  clusterIPs:
  - 10.222.0.10
  internalTrafficPolicy: Cluster
  ipFamilies:
  - IPv4
  ipFamilyPolicy: SingleStack
  ports:
  - name: dns
    port: 53
    protocol: UDP
    targetPort: dns
  - name: dns-tcp
    port: 53
    protocol: TCP
    targetPort: 5353
  selector:
    k8s-app: kube-dns
  sessionAffinity: None
  type: ClusterIP`

	service2YAML = `
---
apiVersion: v1
kind: Service
metadata:
  name: d8-kube-dns-redirect
  namespace: kube-system
spec:
  clusterIP: 10.222.142.22
  clusterIPs:
  - 10.222.142.22
  internalTrafficPolicy: Cluster
  ipFamilies:
  - IPv4
  ipFamilyPolicy: SingleStack
  ports:
  - name: dns
    port: 53
    protocol: UDP
    targetPort: dns
  - name: dns-tcp
    port: 53
    protocol: TCP
    targetPort: dns-tcp
  selector:
    k8s-app: kube-dns
  sessionAffinity: None
  type: ClusterIP`

	serviceRightPorts = `
- name: dns
  port: 53
  protocol: UDP
  targetPort: dns
- name: dns-tcp
  port: 53
  protocol: TCP
  targetPort: dns-tcp`

	serviceBrokenPorts = `
- name: dns
  port: 53
  protocol: UDP
  targetPort: dns
- name: dns-tcp
  port: 53
  protocol: TCP
  targetPort: 5353`
)

var _ = Describe("Global hooks :: migration_service_with_many_ports", func() {
	f := HookExecutionConfigInit("{}", "{}")

	Context("There are broken services and enabled modules", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`["kube-dns"]`))
			f.BindingContexts.Set(f.KubeStateSet(serviceYAML + service2YAML))
			f.RunHook()
		})
		It("Service have been fixed", func() {
			Expect(f).To(ExecuteSuccessfully())
			dnsService := f.KubernetesResource("Service", "kube-system", "d8-kube-dns")

			Expect(dnsService.Field("spec.ports").String()).To(MatchYAML(serviceRightPorts))
		})
	})

	Context("There are broken services and disabled module", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte(`[]`))
			f.BindingContexts.Set(f.KubeStateSet(serviceYAML + service2YAML))
			f.RunHook()
		})
		It("Service have not been fixed", func() {
			Expect(f).To(ExecuteSuccessfully())
			dnsService := f.KubernetesResource("Service", "kube-system", "d8-kube-dns")

			Expect(dnsService.Field("spec.ports").String()).To(MatchYAML(serviceBrokenPorts))
		})
	})
})
