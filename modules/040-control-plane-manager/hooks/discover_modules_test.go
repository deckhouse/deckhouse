/*
Copyright 2021 Flant CJSC

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

var _ = Describe("Modules :: controler-plane-manager :: hooks :: discover_modules ::", func() {
	const (
		configMap = `
---
apiVersion: v1
data:
  url: test
  ca: test
kind: ConfigMap
metadata:
  name: cm
  namespace: d8-user-authz
  labels:
    control-plane-configurator: ""
`
		configMapAdded = `
---
apiVersion: v1
data:
  url: testtest
  ca: testtest
kind: ConfigMap
metadata:
  name: cm
  namespace: d8-user-authz
  labels:
    control-plane-configurator: ""
---
apiVersion: v1
data:
  oidcIssuerURL: test
  oidcIssuerAddress: 8.8.8.8
kind: ConfigMap
metadata:
  name: cm
  namespace: d8-user-authn
  labels:
    control-plane-configurator: ""
`
	)
	const values = `
controlPlaneManager:
  internal: {}
  apiserver:
    authn: {}
    authz: {}
global:
  discovery:
    kubernetesCA: globaltesttest
`

	Context("Empty cluster", func() {
		f := HookExecutionConfigInit(values, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully, but no values should be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").Exists()).ToNot(BeTrue())
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookCA").Exists()).ToNot(BeTrue())
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerURL").Exists()).ToNot(BeTrue())
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcCA").Exists()).ToNot(BeTrue())
		})

		Context("Someone added d8-cloud-instance-manager-cloud-provider", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(configMap))
				f.RunHook()
			})

			It("controlPlaneManager.x values must be filled with data from ConfigMap", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").String()).To(Equal("test"))
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookCA").String()).To(Equal("test"))
			})
		})
	})

	Context("Secret d8-cloud-instance-manager-cloud-provider is in cluster", func() {
		f := HookExecutionConfigInit(values, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(configMap))
			f.RunHook()
		})

		It("controlPlaneManager.x values must be filled with data from ConfigMap", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").String()).To(Equal("test"))
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookCA").String()).To(Equal("test"))
		})

		Context("ConfigMap was added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(configMapAdded))
				f.RunHook()
			})

			It("controlPlaneManager.x values must be filled with data from ConfigMap", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").String()).To(Equal("testtest"))
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookCA").String()).To(Equal("testtest"))
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerURL").String()).To(Equal("test"))
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcCA").String()).To(Equal("globaltesttest"))
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerAddress").String()).To(Equal("8.8.8.8"))
			})

			Context("ConfigMaps were deleted", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(""))
					f.RunHook()
				})

				It("Hook must execute successfully, and all values should be unset", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookCA").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerURL").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcCA").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerAddress").Exists()).ToNot(BeTrue())
				})
			})
		})
	})
})
