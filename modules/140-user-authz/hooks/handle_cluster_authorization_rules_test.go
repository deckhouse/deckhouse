package hooks

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	stateCCRs = `
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: ccr-without-annotation0
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: ccr0
  annotations:
    user-authz.deckhouse.io/access-level: User
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: ccr1
  annotations:
    user-authz.deckhouse.io/access-level: PrivilegedUser
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: ccr2
  annotations:
    user-authz.deckhouse.io/access-level: Editor
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: ccr3
  annotations:
    user-authz.deckhouse.io/access-level: Admin
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: ccr4
  annotations:
    user-authz.deckhouse.io/access-level: ClusterEditor
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: ccr5
  annotations:
    user-authz.deckhouse.io/access-level: ClusterAdmin
`
	stateCARs = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterAuthorizationRule
metadata:
  name: car0
spec:
  accessLevel: ClusterEditor
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterAuthorizationRule
metadata:
  name: car1
spec:
  accessLevel: ClusterAdmin
`
)

var _ = Describe("User Authz hooks :: stores handler ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ClusterAuthorizationRule", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("userAuthz.internal.customClusterRoles must be dict of empty arrays and CAR must empty list", func() {
			ccrExpectation := `
	{
	 "user":[],
	 "privilegedUser":[],
	 "editor":[],
	 "admin":[],
	 "clusterEditor":[],
	 "clusterAdmin":[]
	}`
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles").String()).To(MatchJSON(ccrExpectation))
			Expect(f.ValuesGet("userAuthz.internal.crds").String()).To(MatchJSON(`[]`))
		})
	})

	Context("Cluster with pile of CCRs and two CARs", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCCRs + stateCARs))
			f.RunHook()
		})

		It("CCR and CAR must be stored in values", func() {
			ccrExpectation := `
{
  "user":["ccr0"],
  "privilegedUser":["ccr0","ccr1"],
  "editor":["ccr0","ccr1","ccr2"],
  "admin":["ccr0","ccr1","ccr2","ccr3"],
  "clusterEditor":["ccr0","ccr1","ccr2","ccr4"],
  "clusterAdmin":["ccr0","ccr1","ccr2","ccr3","ccr4","ccr5"]
}`
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles").String()).To(MatchJSON(ccrExpectation))
			Expect(f.ValuesGet("userAuthz.internal.crds").String()).To(MatchJSON(`[{"name":"car0","spec":{"accessLevel":"ClusterEditor"}},{"name":"car1","spec":{"accessLevel":"ClusterAdmin"}}]`))
		})
	})
})
