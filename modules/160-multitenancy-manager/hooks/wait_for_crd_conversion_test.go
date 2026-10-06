/*
Copyright 2026 Flant JSC

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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: multitenancy-manager :: hooks :: wait_for_crd_conversion ::", func() {
	crd := func(name, conversion string) string {
		return fmt.Sprintf(`
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: %s
spec:
  group: example.io
  names: {kind: Example, plural: examples}
  scope: Cluster
  versions:
  - name: v1
    served: true
    storage: true
%s
`, name, conversion)
	}
	const webhook = `  conversion:
    strategy: Webhook
    webhook:
      conversionReviewVersions: [v1]
      clientConfig:
        service: {name: conversion-webhook-handler, namespace: d8-system, path: /x, port: 443}`
	const none = `  conversion:
    strategy: None`

	f := HookExecutionConfigInit(`{"multitenancyManager":{"internal":{}}}`, `{}`)

	Context("every conversion CRD converts through the webhook", func() {
		BeforeEach(func() {
			f.KubeStateSet(crd("clusterresourcegrantpolicies.multitenancy.deckhouse.io", webhook) +
				crd("projects.deckhouse.io", webhook) +
				crd("projecttemplates.deckhouse.io", webhook))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("lets the module run", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("a CRD still has the conversion the API server defaults to", func() {
		BeforeEach(func() {
			f.KubeStateSet(crd("clusterresourcegrantpolicies.multitenancy.deckhouse.io", none) +
				crd("projects.deckhouse.io", webhook) +
				crd("projecttemplates.deckhouse.io", ""))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("holds the module run and names every such CRD", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			Expect(f.GoHookError).To(MatchError(And(
				ContainSubstring("clusterresourcegrantpolicies.multitenancy.deckhouse.io (conversion strategy None)"),
				ContainSubstring("projecttemplates.deckhouse.io (conversion strategy None)"),
				Not(ContainSubstring("projects.deckhouse.io (")),
			)))
		})
	})

	Context("a conversion CRD is not installed", func() {
		BeforeEach(func() {
			f.KubeStateSet(crd("projects.deckhouse.io", webhook))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("does not wait for it", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
})
