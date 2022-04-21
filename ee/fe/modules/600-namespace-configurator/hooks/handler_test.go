/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: namespace-configurator :: hooks :: handler ::", func() {

	Context("Empty config", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)

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
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(0))
		})
		It("Namespace annotations should not change", func() {
			ns := f.KubernetesResource("Namespace", "", "test1")
			Expect(ns.Field(`metadata.annotations.extended-monitoring\.flant\.com/enabled`).Exists()).To(BeFalse())
			ns = f.KubernetesResource("Namespace", "", "test2")
			Expect(ns.Field(`metadata.annotations.extended-monitoring\.flant\.com/enabled`).Exists()).To(BeTrue())
		})
	})

	Context("Patch cases", func() {

		f := HookExecutionConfigInit(`{"namespaceConfigurator":{"configurations":[{"annotations":{"some":null},"labels":{"foo":"bar","bee":null},"includeNames":["test1", "test2", "test3", "test4", "test6"],"excludeNames":["test2"]}]}}`, `{}`)

		BeforeEach(func() {
			f.KubeStateSet(`
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
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Expected patch", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(3))
		})
		It("Namespace annotations should change", func() {
			ns := f.KubernetesResource("Namespace", "", "test1")
			Expect(ns.Field(`metadata.annotations.some`).Exists()).To(BeFalse())
		})
		It("Namespace labels should change", func() {
			ns := f.KubernetesResource("Namespace", "", "test1")
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
		})
		It("Namespace labels should not change", func() {
			ns := f.KubernetesResource("Namespace", "", "test2")
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

		f := HookExecutionConfigInit(`{"namespaceConfigurator":{"configurations":[{"annotations":{"extended-monitoring.flant.com/enabled":"true"},"includeNames":["prod-.*","infra-.*"],"excludeNames":["infra-test"]}]}}`, `{}`)

		BeforeEach(func() {
			f.KubeStateSet(`
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
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Expect patch", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(2))
		})
		It("Namespace annotations should change", func() {
			ns := f.KubernetesResource("Namespace", "", "prod-ns1")
			Expect(ns.Field(`metadata.annotations.extended-monitoring\.flant\.com/enabled`).Exists()).To(BeTrue())
			ns = f.KubernetesResource("Namespace", "", "infra-ns1")
			Expect(ns.Field(`metadata.annotations.extended-monitoring\.flant\.com/enabled`).Exists()).To(BeTrue())
		})
		It("Namespace annotations should not change", func() {
			ns := f.KubernetesResource("Namespace", "", "foo")
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
