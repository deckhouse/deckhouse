/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: operator-trivy :: hooks :: discover labeled namespaces ::", func() {
	f := HookExecutionConfigInit(`{"operatorTrivy":{"internal": {"enabledNamespaces": []}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(enabledNamespacesValuesPath).String()).To(MatchJSON(`[]`))
		})
	})

	Context("Cluster with namespaces", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: v1
kind: Namespace
metadata:
  name: test2
  labels:
    security-scanning.deckhouse.io/enabled: ""
---
apiVersion: v1
kind: Namespace
metadata:
  name: test1
  labels:
    security-scanning.deckhouse.io/enabled: ""
---
apiVersion: v1
kind: Namespace
metadata:
  name: test3
---
apiVersion: v1
kind: Namespace
metadata:
  name: test4
  labels:
    test: test
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Values must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(enabledNamespacesValuesPath).String()).To(MatchJSON(`["test1","test2"]`))
		})
	})
})
