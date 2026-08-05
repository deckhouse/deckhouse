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
1. There is Deployment 'deckhouse' in ns 'd8-system'. Hook must parse registry url from `.spec.template.spec.containers[0].image` and store it to `global.modulesImages.registry`.
2. There is Secret 'deckhouse-registry' in ns 'd8-system'. Hook must parse `.data.".dockerconfigjson"` and store it to `global.modulesImages.registry.dockercfg`.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: deckhouse_registry ::", func() {
	const (
		initValuesString       = `{"global": {"modulesImages": { "registry": {}}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateDeckhouseRegistrySecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-registry
  namespace: d8-system
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: eHl6Cg==
  scheme: aHR0cA==                  # http
  ca: Q0FDQUNB                      # CACACA
  address: cmVnaXN0cnkudGVzdC5jb20= # registry.test.com
  path: L2RlY2tob3VzZQ==            # /deckhouse
`

		stateDeckhouseRegistrySecretWithoutCAandScheme = `
---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-registry
  namespace: d8-system
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: eHl6Cg==
  address: cmVnaXN0cnkudGVzdC5jb20= # registry.test.com
  path: L2RlY2tob3VzZQ==            # /deckhouse
`

		stateDeckhouseRegistrySecretWithoutAddress = `
---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-registry
  namespace: d8-system
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: eHl6Cg==
  path: L2RlY2tob3VzZQ==            # /deckhouse
`

		stateDeckhouseRegistrySecretWithoutDockerconfigjson = `
---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-registry
  namespace: d8-system
type: kubernetes.io/dockerconfigjson
data:
  address: cmVnaXN0cnkudGVzdC5jb20= # registry.test.com
  path: L2RlY2tob3VzZQ==            # /deckhouse
`

		statePublishedImageAddress = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: registry-image-address
  namespace: d8-system
data:
  base: registry.d8-system.svc:5001/system/deckhouse
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	Context("Secret is created", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeckhouseRegistrySecret))
			f.RunHook()
		})

		It("Values must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.modulesImages.registry.base").String()).To(Equal("registry.test.com/deckhouse"))
			Expect(f.ValuesGet("global.modulesImages.registry.dockercfg").String()).To(Equal("eHl6Cg=="))
			Expect(f.ValuesGet("global.modulesImages.registry.CA").String()).To(Equal("CACACA"))
			Expect(f.ValuesGet("global.modulesImages.registry.scheme").String()).To(Equal("http"))
			Expect(f.ValuesGet("global.modulesImages.registry.address").String()).To(Equal("registry.test.com"))
			Expect(f.ValuesGet("global.modulesImages.registry.path").String()).To(Equal("/deckhouse"))
		})
	})

	Context("Secret without CA and Scheme is created", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeckhouseRegistrySecretWithoutCAandScheme))
			f.RunHook()
		})

		It("Values must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.modulesImages.registry.base").String()).To(Equal("registry.test.com/deckhouse"))
			Expect(f.ValuesGet("global.modulesImages.registry.dockercfg").String()).To(Equal("eHl6Cg=="))
			Expect(f.ValuesGet("global.modulesImages.registry.CA").String()).To(BeEmpty())
			Expect(f.ValuesGet("global.modulesImages.registry.scheme").String()).To(Equal("https"))
			Expect(f.ValuesGet("global.modulesImages.registry.address").String()).To(Equal("registry.test.com"))
			Expect(f.ValuesGet("global.modulesImages.registry.path").String()).To(Equal("/deckhouse"))
		})
	})

	// The registry module can move where image references point, and only that.
	//
	// Everything else here describes the registry the cluster was installed with, and has
	// to keep describing it: those values are read by the Deckhouse controller's own HTTP
	// client — the release check and the default module source — which has no node agent
	// in its path and cannot reach an in-cluster address. Image references are resolved by
	// the container runtime, which does.
	Context("The registry module has published an image address", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeckhouseRegistrySecret + statePublishedImageAddress))
			f.RunHook()
		})

		It("renders image references from it, and leaves the rest on the upstream registry", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.modulesImages.registry.base").String()).
				To(Equal("registry.d8-system.svc:5001/system/deckhouse"))

			Expect(f.ValuesGet("global.modulesImages.registry.address").String()).To(Equal("registry.test.com"))
			Expect(f.ValuesGet("global.modulesImages.registry.path").String()).To(Equal("/deckhouse"))
			Expect(f.ValuesGet("global.modulesImages.registry.scheme").String()).To(Equal("http"))
			Expect(f.ValuesGet("global.modulesImages.registry.CA").String()).To(Equal("CACACA"))
			Expect(f.ValuesGet("global.modulesImages.registry.dockercfg").String()).To(Equal("eHl6Cg=="))
		})
	})

	// Absent is the normal case, and the one every cluster upgrading into this must land
	// in: the module not managing the pull path, the module still on its previous
	// implementation, and node agents that have not applied their layout yet all leave it
	// unpublished, and nothing about how the cluster pulls changes.
	Context("Nothing has been published", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeckhouseRegistrySecret))
			f.RunHook()
		})

		It("renders image references from the registry the cluster was installed with", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.modulesImages.registry.base").String()).To(Equal("registry.test.com/deckhouse"))
		})
	})

	Context("Secret without Address is created", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeckhouseRegistrySecretWithoutAddress))
			f.RunHook()
		})

		It("Should fail", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			Expect(f.GoHookError).To(MatchError("address field not found in 'deckhouse-registry' secret"))
		})
	})

	Context("Secret without Dockerconfigjson is created", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeckhouseRegistrySecretWithoutDockerconfigjson))
			f.RunHook()
		})

		It("Should fail", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			Expect(f.GoHookError).To(MatchError("docker config not found in 'deckhouse-registry' secret"))
		})
	})

})
