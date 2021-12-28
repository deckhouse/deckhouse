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
2. There is Secret 'deckhouse-registry' in ns 'd8-system'. Hook must parse `.data.".dockerconfigjson"` and store it to `global.modulesImages.registryDockercfg`.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: deckhouse_registry ::", func() {
	const (
		initValuesString       = `{"global": {"modulesImages": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateDeployAndSecret = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deckhouse
  namespace: d8-system
spec:
  template:
    spec:
      containers:
      - name: deckhouse
        image: registry.example.com/developers/deckhouse/dev:dashboard-spare-domain-fix
---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-registry
  namespace: d8-system
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: eHl6Cg==
`
		stateDeployAndSecretWithSHA = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deckhouse
  namespace: d8-system
spec:
  template:
    spec:
      containers:
      - name: deckhouse
        image: registry.example.com/developers/deckhouse/dev:dashboard-spare-domain-fix@sha256:abcdefg
---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-registry
  namespace: d8-system
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: eHl6Cg==
`
		stateDeployOnly = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deckhouse
  namespace: d8-system
spec:
  template:
    spec:
      containers:
      - name: deckhouse
        image: registry.example.com/developers/deckhouse/dev:dashboard-spare-domain-fix
`
		stateDeployAndSecretWithRegistryParamsSchemeAndCA = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deckhouse
  namespace: d8-system
spec:
  template:
    spec:
      containers:
      - name: deckhouse
        image: registry.example.com:8080/developers/deckhouse/dev:dashboard-spare-domain-fix
---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-registry
  namespace: d8-system
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: eHl6Cg==
  scheme: aHR0cA== # http
  ca: Q0FDQUNB     # CACACA
`
		stateDeployAndSecretWithRegistryParamsAddressAndSchemeAndCA = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deckhouse
  namespace: d8-system
spec:
  template:
    spec:
      containers:
      - name: deckhouse
        image: registry.example.com:8080/developers/deckhouse/dev:dashboard-spare-domain-fix
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
`
		stateDeployAndSecretWithRegistryParamsAddressAndPathAndSchemeAndCA = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deckhouse
  namespace: d8-system
spec:
  template:
    spec:
      containers:
      - name: deckhouse
        image: registry.example.com:8080/developers/deckhouse/dev:dashboard-spare-domain-fix
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

		Context("Deployment and Secret are created", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateDeployAndSecret))
				f.RunHook()
			})

			It("Values must be set", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.modulesImages.registry").String()).To(Equal("registry.example.com/developers/deckhouse"))
				Expect(f.ValuesGet("global.modulesImages.registryDockercfg").String()).To(Equal("eHl6Cg=="))
				Expect(f.ValuesGet("global.modulesImages.registryCA").String()).To(BeEmpty())
				Expect(f.ValuesGet("global.modulesImages.registryScheme").String()).To(Equal("https"))
				Expect(f.ValuesGet("global.modulesImages.registryAddress").String()).To(Equal("registry.example.com"))
				Expect(f.ValuesGet("global.modulesImages.registryPath").String()).To(Equal("/developers/deckhouse"))
			})
		})
	})

	Context("Deployment and Secret are in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeployAndSecret))
			f.RunHook()
		})

		It("Values must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.modulesImages.registry").String()).To(Equal("registry.example.com/developers/deckhouse"))
			Expect(f.ValuesGet("global.modulesImages.registryDockercfg").String()).To(Equal("eHl6Cg=="))
			Expect(f.ValuesGet("global.modulesImages.registryCA").String()).To(BeEmpty())
			Expect(f.ValuesGet("global.modulesImages.registryScheme").String()).To(Equal("https"))
			Expect(f.ValuesGet("global.modulesImages.registryAddress").String()).To(Equal("registry.example.com"))
			Expect(f.ValuesGet("global.modulesImages.registryPath").String()).To(Equal("/developers/deckhouse"))
		})

	})

	Context("Deployment with sha256 and Secret are in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeployAndSecretWithSHA))
			f.RunHook()
		})

		It("Values must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.modulesImages.registry").String()).To(Equal("registry.example.com/developers/deckhouse"))
			Expect(f.ValuesGet("global.modulesImages.registryDockercfg").String()).To(Equal("eHl6Cg=="))
			Expect(f.ValuesGet("global.modulesImages.registryCA").String()).To(BeEmpty())
			Expect(f.ValuesGet("global.modulesImages.registryScheme").String()).To(Equal("https"))
			Expect(f.ValuesGet("global.modulesImages.registryAddress").String()).To(Equal("registry.example.com"))
			Expect(f.ValuesGet("global.modulesImages.registryPath").String()).To(Equal("/developers/deckhouse"))
		})
	})

	Context("Deployment and Secret with CA and scheme set are in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeployAndSecretWithRegistryParamsSchemeAndCA))
			f.RunHook()
		})

		It("Values must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.modulesImages.registry").String()).To(Equal("registry.example.com:8080/developers/deckhouse"))
			Expect(f.ValuesGet("global.modulesImages.registryDockercfg").String()).To(Equal("eHl6Cg=="))
			Expect(f.ValuesGet("global.modulesImages.registryCA").String()).To(Equal("CACACA"))
			Expect(f.ValuesGet("global.modulesImages.registryScheme").String()).To(Equal("http"))
			Expect(f.ValuesGet("global.modulesImages.registryAddress").String()).To(Equal("registry.example.com:8080"))
			Expect(f.ValuesGet("global.modulesImages.registryPath").String()).To(Equal("/developers/deckhouse"))
		})
	})

	Context("Deployment and Secret with Address, CA and scheme set are in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeployAndSecretWithRegistryParamsAddressAndSchemeAndCA))
			f.RunHook()
		})

		It("Values must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.modulesImages.registry").String()).To(Equal("registry.example.com:8080/developers/deckhouse"))
			Expect(f.ValuesGet("global.modulesImages.registryDockercfg").String()).To(Equal("eHl6Cg=="))
			Expect(f.ValuesGet("global.modulesImages.registryCA").String()).To(Equal("CACACA"))
			Expect(f.ValuesGet("global.modulesImages.registryScheme").String()).To(Equal("http"))
			Expect(f.ValuesGet("global.modulesImages.registryAddress").String()).To(Equal("registry.test.com"))
		})
	})

	Context("Deployment and Secret with Address, Path, CA and scheme set are in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeployAndSecretWithRegistryParamsAddressAndPathAndSchemeAndCA))
			f.RunHook()
		})

		It("Values must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.modulesImages.registry").String()).To(Equal("registry.example.com:8080/developers/deckhouse"))
			Expect(f.ValuesGet("global.modulesImages.registryDockercfg").String()).To(Equal("eHl6Cg=="))
			Expect(f.ValuesGet("global.modulesImages.registryCA").String()).To(Equal("CACACA"))
			Expect(f.ValuesGet("global.modulesImages.registryScheme").String()).To(Equal("http"))
			Expect(f.ValuesGet("global.modulesImages.registryAddress").String()).To(Equal("registry.test.com"))
			Expect(f.ValuesGet("global.modulesImages.registryPath").String()).To(Equal("/deckhouse"))
		})
	})

	Context("Secret was deleted", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeployOnly))
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

})
