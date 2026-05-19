// Copyright 2026 Flant JSC
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

User-stories: Hook must discover if there is a default Gateway API Gateway in the cluster and save its name and namespace to .Values.global.discovery.gatewayAPIDefaultGateway

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	albGatewayNamespace = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-alb
`
	emptyDefaultGatewayCM = `
---
apiVersion: v1
data:
kind: ConfigMap
metadata:
  labels:
    app: d8-alb
    heritage: deckhouse
  name: default-gateway
  namespace: d8-alb
`

	defaultGatewayCM = `
---
apiVersion: v1
data:
  defaultGateway: d8-alb/shared-gateway
kind: ConfigMap
metadata:
  labels:
    app: d8-alb
    heritage: deckhouse
  name: default-gateway
  namespace: d8-alb
`
)

var _ = Describe("Global hooks :: discovery :: default_gateway ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	Context("Empty discovery data", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		Context("Empty cluster", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(``))
				f.RunHook()
			})

			It("Must be executed successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("`global.discovery.gatewayAPIDefaultGateway` must be set empty", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("global.discovery.gatewayAPIDefaultGateway").Exists()).To(BeTrue())
				Expect(f.ValuesGet("global.discovery.gatewayAPIDefaultGateway")).To(MatchJSON(`{"name": "", "namespace": ""}`))
			})
		})

		Context("ALB-Gateway namespace exists, but no configmap", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(albGatewayNamespace))
				f.RunHook()
			})

			It("Must be executed successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("`global.discovery.gatewayAPIDefaultGateway` must be set empty", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("global.discovery.gatewayAPIDefaultGateway").Exists()).To(BeTrue())
				Expect(f.ValuesGet("global.discovery.gatewayAPIDefaultGateway")).To(MatchJSON(`{"name": "", "namespace": ""}`))
			})
		})

		Context("ALB-Gateway namespace and empty configmap exist", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(albGatewayNamespace + emptyDefaultGatewayCM))
				f.RunHook()
			})

			It("Must be executed successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("`global.discovery.gatewayAPIDefaultGateway` must be set empty", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("global.discovery.gatewayAPIDefaultGateway").Exists()).To(BeTrue())
				Expect(f.ValuesGet("global.discovery.gatewayAPIDefaultGateway")).To(MatchJSON(`{"name": "", "namespace": ""}`))
			})
		})

		Context("ALB-Gateway namespace and configmap exist", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(albGatewayNamespace + defaultGatewayCM))
				f.RunHook()
			})

			It("Must be executed successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("`global.discovery.gatewayAPIDefaultGateway` must be set", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("global.discovery.gatewayAPIDefaultGateway").Exists()).To(BeTrue())
				Expect(f.ValuesGet("global.discovery.gatewayAPIDefaultGateway")).To(MatchJSON(`{"name": "shared-gateway", "namespace": "d8-alb"}`))
			})
		})
	})

	Context("Global discovery data is provided", func() {
		f := HookExecutionConfigInit(`{"global": {"discovery": {"gatewayAPIDefaultGateway": {"name": "mygateway", "namespace": "default"}}}}`, initConfigValuesString)

		Context("default gateway is updated with data from the configmap", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(albGatewayNamespace + defaultGatewayCM))
				f.RunHook()
			})

			It("Must be executed successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("`global.discovery.gatewayAPIDefaultGateway` must be updated", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("global.discovery.gatewayAPIDefaultGateway").Exists()).To(BeTrue())
				Expect(f.ValuesGet("global.discovery.gatewayAPIDefaultGateway")).To(MatchJSON(`{"name": "shared-gateway", "namespace": "d8-alb"}`))
			})
		})

		Context("default gateway is missing and deleted from discovery", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(albGatewayNamespace + emptyDefaultGatewayCM))
				f.RunHook()
			})

			It("Must be executed successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("`global.discovery.gatewayAPIDefaultGateway` must be set empty", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("global.discovery.gatewayAPIDefaultGateway").Exists()).To(BeTrue())
				Expect(f.ValuesGet("global.discovery.gatewayAPIDefaultGateway")).To(MatchJSON(`{"name": "", "namespace": ""}`))
			})
		})
	})
})
