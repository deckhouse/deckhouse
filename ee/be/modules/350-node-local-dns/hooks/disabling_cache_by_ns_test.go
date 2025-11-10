/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString = `{
  "global": {
    "discovery": {
      "clusterDomain": "cluster.local"
    }
  },
  "nodeLocalDns": {
    "internal": {}
  }
}`
	initConfigValuesString = `{}`
	namespacesKubeState    = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: dev
  labels:
    node-local-dns.deckhouse.io/disable-cache: ""
---
apiVersion: v1
kind: Namespace
metadata:
  name: prod
  labels:
    node-local-dns.deckhouse.io/disable-cache: ""
`
)

var _ = Describe("Modules :: node local dns :: hooks :: disabling cache by namespaces ::", func() {

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("No namespaces labelled to disable cache", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should store empty list of disabled zones", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeLocalDns.internal.disabledCache.zones").AsStringSlice()).To(BeEmpty())
		})
	})

	Context("Several namespaces request cache disabling", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(namespacesKubeState))
			f.RunHook()
		})

		It("Should store zones derived from namespaces and clusterDomain", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeLocalDns.internal.disabledCache.zones").AsStringSlice()).
				To(Equal([]string{"dev.cluster.local", "prod.cluster.local"}))
		})
	})
})
