/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString       = `{"global": {}, "userAuthz": {"enableMultiTenancy": true, "internal": {"multitenancyCRDs": []}}}`
	initConfigValuesString = `{}`
)

var _ = FDescribe("User-authz :: role bindings ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("deckhouse.io", "v1", "ClusterAuthorizationRule", false)

	Context("Cluster has no nodes except master", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(defaultNS + rule1))
			f.RunHook()
		})

		It("`global.clusterIsBootstrapped` must not exist", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.clusterIsBootstrapped").Exists()).To(BeFalse())
			fmt.Println(f.ValuesGet("userAuthz.internal.multitenancyCRDs"))
		})

	})
})

const (
	defaultNS = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: review-1
---
apiVersion: v1
kind: Namespace
metadata:
  name: default
`

	rule1 = `
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  subjects:
  - kind: User
    name: user@flant.com
  accessLevel: SuperAdmin
  allowAccessToSystemNamespaces: true
  limitNamespaces:
    - review-.*
`
)
