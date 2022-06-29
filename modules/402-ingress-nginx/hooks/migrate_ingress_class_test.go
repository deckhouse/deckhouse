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

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: minimal_ingress_version ", func() {
	initValuesString := `{"ingressNginx":{"defaultControllerVersion": "0.33", "internal": {"ingressControllers": []}}}`
	globalValuesString := `{}`
	f := HookExecutionConfigInit(initValuesString, globalValuesString)
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", false)

	Context("No existing ingress classes", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("ingressNginx.internal.ingressControllers", []byte(`
[
	{
		"name": "test",
		"spec": {
		  "ingressClass": "nginx",
		  "controllerVersion": "0.33"
		}
	}
]
`))
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster has up to date IngressClass", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("ingressNginx.internal.ingressControllers", []byte(`
[
	{
		"name": "test",
		"spec": {
		  "ingressClass": "nginx",
		  "controllerVersion": "0.33"
		}
	}
]
`))
			f.KubeStateSet(`
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  labels:
    heritage: deckhouse
    module: ingress-nginx
  name: nginx
spec:
  controller: k8s.io/ingress-nginx
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should have kept IngressClass", func() {
			Expect(f).To(ExecuteSuccessfully())
			ingC := f.KubernetesGlobalResource("IngressClass", "nginx")
			Expect(ingC.Exists()).To(BeTrue())
		})
	})

	Context("Cluster has outdated IngressClass", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("ingressNginx.internal.ingressControllers", []byte(`
[
	{
		"name": "test",
		"spec": {
		  "ingressClass": "nginx",
		  "controllerVersion": "1.1"
		}
	}
]
`))
			f.KubeStateSet(`
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  labels:
    heritage: deckhouse
    module: ingress-nginx
  name: nginx
spec:
  controller: k8s.io/ingress-nginx
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should delete IngressClass", func() {
			Expect(f).To(ExecuteSuccessfully())
			ingC := f.KubernetesGlobalResource("IngressClass", "nginx")
			Expect(ingC.Exists()).To(BeFalse())
		})
	})

	Context("controller version not set. Rollback to 0.33", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("ingressNginx.internal.ingressControllers", []byte(`
[
	{
		"name": "test",
		"spec": {
		  "ingressClass": "nginx"
		}
	}
]
`))
			f.KubeStateSet(`
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  labels:
    heritage: deckhouse
    module: ingress-nginx
  name: nginx
spec:
  controller: ingress-nginx.deckhouse.io/nginx
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should delete IngressClass", func() {
			Expect(f).To(ExecuteSuccessfully())
			ingC := f.KubernetesGlobalResource("IngressClass", "nginx")
			Expect(ingC.Exists()).To(BeFalse())
		})
	})
})
