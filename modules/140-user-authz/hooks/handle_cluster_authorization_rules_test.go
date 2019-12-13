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
	stateCCR0andCAR0 = `
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: ccr0
  annotations:
    user-authz.deckhouse.io/access-level: ClusterEditor
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: ccr-without-annotation0
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterAuthorizationRule
metadata:
  name: car0
spec:
  accessLevel: ClusterEditor
`
	stateCCR0andCAR0Modified = `
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: ccr0
  annotations:
    user-authz.deckhouse.io/access-level: ClusterAdmin
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: ccr-without-annotation0
  labels:
    fake: fake
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterAuthorizationRule
metadata:
  name: car0
spec:
  accessLevel: ClusterAdmin
`
	stateCCR0CCR1andCAR0CAR1 = `
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: ccr0
  annotations:
    user-authz.deckhouse.io/access-level: ClusterEditor
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: ccr-without-annotation0
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterAuthorizationRule
metadata:
  name: car0
spec:
  accessLevel: ClusterEditor
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: ccr1
  annotations:
    user-authz.deckhouse.io/access-level: ClusterAdmin
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: ccr-without-annotation1
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterAuthorizationRule
metadata:
  name: car1
spec:
  accessLevel: ClusterAdmin
`
)

const largeClusterRolesStore = `
[
  {"name":"ccr0","accessLevel":"user"},
  {"name":"ccr1","accessLevel":"privilegedUser"},
  {"name":"ccr2","accessLevel":"editor"},
  {"name":"ccr3","accessLevel":"admin"},
  {"name":"ccr4","accessLevel":"clusterEditor"},
  {"name":"ccr5","accessLevel":"clusterAdmin"}
]
`

var _ = Describe("User Authz hooks :: stores handler ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ClusterAuthorizationRule", false)

	Context("Cluster with CCR and CAR", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCCR0andCAR0)...)
			f.RunHook()
		})

		It("CCR and CAR must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.customClusterRolesStore").String()).To(Equal(`[{"name":"ccr0","accessLevel":"clusterEditor"}]`))
			Expect(f.ValuesGet("userAuthz.internal.crds").String()).To(Equal(`[{"name":"car0","spec":{"accessLevel":"ClusterEditor"}}]`))
		})

		Context("Both CCR and CAR are modified", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateCCR0andCAR0Modified)...)
				f.RunHook()
			})

			It("Modified CCR and CAR must be stored in values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("userAuthz.internal.customClusterRolesStore").String()).To(Equal(`[{"name":"ccr0","accessLevel":"clusterAdmin"}]`))
				Expect(f.ValuesGet("userAuthz.internal.crds").String()).To(Equal(`[{"name":"car0","spec":{"accessLevel":"ClusterAdmin"}}]`))
			})
		})

		Context("Extra CCR and CAR added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateCCR0CCR1andCAR0CAR1)...)
				f.RunHook()
			})

			It("Extra CCR and CAR must be stored in values with original CCR and CAR", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("userAuthz.internal.customClusterRolesStore").String()).To(Equal(`[{"name":"ccr0","accessLevel":"clusterEditor"},{"name":"ccr1","accessLevel":"clusterAdmin"}]`))
				Expect(f.ValuesGet("userAuthz.internal.crds").String()).To(Equal(`[{"name":"car0","spec":{"accessLevel":"ClusterEditor"}},{"name":"car1","spec":{"accessLevel":"ClusterAdmin"}}]`))
			})

			Context("Extra CCR and CAR deleted", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateCCR0andCAR0)...)
					f.RunHook()
				})

				It("Original CCR and CAR must be stored in values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("userAuthz.internal.customClusterRolesStore").String()).To(Equal(`[{"name":"ccr0","accessLevel":"clusterEditor"}]`))
					Expect(f.ValuesGet("userAuthz.internal.crds").String()).To(Equal(`[{"name":"car0","spec":{"accessLevel":"ClusterEditor"}}]`))
				})
			})
		})
	})

	Context("userAuthz.internal.customClusterRolesStore is set to large array and onBeforeHelm hook fired", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.ValuesSet("userAuthz.internal.customClusterRolesStore", []byte(largeClusterRolesStore))
			f.RunHook()
		})

		It("userAuthz.internal.customClusterRoles must be calculated to flat version of customClusterRolesStore", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles").String()).To(Equal(`{"user":["ccr0"],"privilegedUser":["ccr0","ccr1"],"editor":["ccr0","ccr1","ccr2"],"admin":["ccr0","ccr1","ccr2","ccr3"],"clusterEditor":["ccr0","ccr1","ccr2","ccr4"],"clusterAdmin":["ccr0","ccr1","ccr2","ccr3","ccr4","ccr5"]}`))
		})
	})
})
