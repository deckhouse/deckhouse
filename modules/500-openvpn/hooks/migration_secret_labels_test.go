/*
Copyright 2025 Flant JSC

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

var _ = Describe("Modules :: openvpn :: hooks :: migrate_secret_labels", func() {

	const (
		d8OpenvpnNamespace = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-openvpn
  labels:
    heritage: deckhouse
    module: openvpn
spec:
  finalizers:
  - kubernetes
`
		validServerCertSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: openvpn-pki-server
  namespace: d8-openvpn
  labels:
    index.txt: ""
    name: server
    type: serverAuth

type: Opaque
data: {}
`
		invalidServerCertSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: openvpn-pki-server
  namespace: d8-openvpn
  labels:
    type: serverAuth

type: Opaque
data: {}
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("An empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.KubeStateSet(``)
			f.RunGoHook()
		})

		It("Hook is executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with no labeled cert", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(d8OpenvpnNamespace + invalidServerCertSecret))
			f.RunGoHook()
		})

		It("should appear label name 'server'", func() {
			Expect(f).To(ExecuteSuccessfully())
			labels := f.KubernetesResource("Secret", "d8-openvpn", "openvpn-pki-server").Field("metadata.labels")
			Expect(labels.Get("name").String()).To(Equal("server"))
		})
	})

	Context("Cluster with labeled cert", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(d8OpenvpnNamespace + validServerCertSecret))
			f.RunGoHook()
		})

		It("should exist label name 'server'", func() {
			Expect(f).To(ExecuteSuccessfully())
			labels := f.KubernetesResource("Secret", "d8-openvpn", "openvpn-pki-server").Field("metadata.labels")
			Expect(labels.Get("name").String()).To(Equal("server"))
			Expect(labels.Get("index.txt").Exists())
		})
	})
})
