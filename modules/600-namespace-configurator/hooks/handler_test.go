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

var _ = Describe("Modules :: namespace-configurator :: hooks :: handler ::", func() {
	f := HookExecutionConfigInit(`{"namespaceConfigurator":{}}`, `{}`)

	Context("Empty config", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: v1
kind: Namespace
metadata:
  name: test1
---
apiVersion: v1
kind: Namespace
metadata:
  name: test2
  annotations:
    extended-monitoring.flant.com/enabled: "true"
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Expected patch", func() {
			ns := f.KubernetesResource("Namespace", "", "test1")
			Expect(ns.Field(`metadata.annotations.extended-monitoring\.flant\.com/enabled`).Exists()).To(BeFalse())
			ns = f.KubernetesResource("Namespace", "", "test2")
			Expect(ns.Field(`metadata.annotations.extended-monitoring\.flant\.com/enabled`).Exists()).To(BeTrue())
		})

		Context("Adding new config", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("namespaceConfigurator", []byte(`
---
configurations:
  - annotations:
      extended-monitoring.flant.com/enabled: "true"
    includeNames: ["test1"]
`))
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
			})
			It("Expected patch", func() {
				ns := f.KubernetesResource("Namespace", "", "test1")
				Expect(ns.Field(`metadata.annotations.extended-monitoring\.flant\.com/enabled`).Exists()).To(BeTrue())
				ns = f.KubernetesResource("Namespace", "", "test2")
				Expect(ns.Field(`metadata.annotations.extended-monitoring\.flant\.com/enabled`).Exists()).To(BeTrue())
			})

		})
	})
	Context("Patch cases", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("namespaceConfigurator", []byte(`
---
configurations:
  - annotations: {"some":null}
    labels: {"foo":"bar","bee":null}
    includeNames: ["test1", "test2", "test3", "test4", "test6"]
    excludeNames: ["test2"]
`))
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    some: val
  name: test1
---
apiVersion: v1
kind: Namespace
metadata:
  name: test2
  annotations:
    some: val
  labels:
    foo: bar
---
apiVersion: v1
kind: Namespace
metadata:
  name: test3
  labels:
    foo: baz
---
apiVersion: v1
kind: Namespace
metadata:
  name: test4
---
apiVersion: v1
kind: Namespace
metadata:
  name: test5
  labels:
    foo: baz
---
apiVersion: v1
kind: Namespace
metadata:
  name: test6
  labels:
    foo: bar
`))
			f.RunHook()
		})

		It("Expected patch", func() {
			ns := f.KubernetesResource("Namespace", "", "test1")
			Expect(ns.Field(`metadata.annotations.some`).Exists()).To(BeFalse())

			ns = f.KubernetesResource("Namespace", "", "test1")
			Expect(ns.Field(`metadata.labels.foo`).Exists()).To(BeTrue())
			Expect(ns.Field("metadata.labels.foo").String()).To(Equal(`bar`))
			Expect(ns.Field(`metadata.labels.bee`).Exists()).To(BeFalse())

			ns = f.KubernetesResource("Namespace", "", "test3")
			Expect(ns.Field(`metadata.labels.foo`).Exists()).To(BeTrue())
			Expect(ns.Field("metadata.labels.foo").String()).To(Equal(`bar`))
			Expect(ns.Field(`metadata.labels.bee`).Exists()).To(BeFalse())
			ns = f.KubernetesResource("Namespace", "", "test4")
			Expect(ns.Field(`metadata.labels.foo`).Exists()).To(BeTrue())
			Expect(ns.Field("metadata.labels.foo").String()).To(Equal(`bar`))
			Expect(ns.Field(`metadata.labels.bee`).Exists()).To(BeFalse())

			ns = f.KubernetesResource("Namespace", "", "test2")
			Expect(ns.Field(`metadata.labels.foo`).Exists()).To(BeTrue())
			Expect(ns.Field("metadata.labels.foo").String()).To(Equal(`bar`))
			Expect(ns.Field(`metadata.labels.bee`).Exists()).To(BeFalse())
			ns = f.KubernetesResource("Namespace", "", "test5")
			Expect(ns.Field(`metadata.labels.foo`).Exists()).To(BeTrue())
			Expect(ns.Field("metadata.labels.foo").String()).To(Equal(`baz`))
			Expect(ns.Field(`metadata.labels.bee`).Exists()).To(BeFalse())
			ns = f.KubernetesResource("Namespace", "", "test6")
			Expect(ns.Field(`metadata.labels.foo`).Exists()).To(BeTrue())
			Expect(ns.Field("metadata.labels.foo").String()).To(Equal(`bar`))
			Expect(ns.Field(`metadata.labels.bee`).Exists()).To(BeFalse())
		})
	})

	Context("Pattern matching", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("namespaceConfigurator", []byte(`
---
configurations:
  - annotations:
      extended-monitoring.flant.com/enabled: "true"
    includeNames: ["prod-.*","infra-.*"]
    excludeNames: ["infra-test"]
`))
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Namespace
metadata:
  name: foo
---
apiVersion: v1
kind: Namespace
metadata:
  name: prod-ns1
---
apiVersion: v1
kind: Namespace
metadata:
  name: infra-ns1
---
apiVersion: v1
kind: Namespace
metadata:
  name: infra-test
---
apiVersion: v1
kind: Namespace
metadata:
  name: infra-test2
  annotations:
    extended-monitoring.flant.com/enabled: "true"
---
apiVersion: v1
kind: Namespace
metadata:
  name: infra-test3
  labels:
    heritage: upmeter
---
apiVersion: v1
kind: Namespace
metadata:
  name: prod-ns2
  annotations:
    extended-monitoring.flant.com/enabled: "true"
`))
			f.RunHook()
		})

		It("Expected patch", func() {
			ns := f.KubernetesResource("Namespace", "", "prod-ns1")
			Expect(ns.Field(`metadata.annotations.extended-monitoring\.flant\.com/enabled`).Exists()).To(BeTrue())
			ns = f.KubernetesResource("Namespace", "", "infra-ns1")
			Expect(ns.Field(`metadata.annotations.extended-monitoring\.flant\.com/enabled`).Exists()).To(BeTrue())

			ns = f.KubernetesResource("Namespace", "", "foo")
			Expect(ns.Field(`metadata.annotations.extended-monitoring\.flant\.com/enabled`).Exists()).To(BeFalse())
			ns = f.KubernetesResource("Namespace", "", "infra-test")
			Expect(ns.Field(`metadata.annotations.extended-monitoring\.flant\.com/enabled`).Exists()).To(BeFalse())
			ns = f.KubernetesResource("Namespace", "", "infra-test2")
			Expect(ns.Field(`metadata.annotations.extended-monitoring\.flant\.com/enabled`).Exists()).To(BeTrue())
			ns = f.KubernetesResource("Namespace", "", "prod-ns2")
			Expect(ns.Field(`metadata.annotations.extended-monitoring\.flant\.com/enabled`).Exists()).To(BeTrue())
			ns = f.KubernetesResource("Namespace", "", "infra-test3")
			Expect(ns.Field(`metadata.annotations.extended-monitoring\.flant\.com/enabled`).Exists()).To(BeFalse())
		})
	})
})
