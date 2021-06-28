package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: get dex user crds ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "User", false)

	Context("User expiration schedule", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  groups:
  - Admins
  - Everyone
  password: password
  ttl: 60m
status:
  expireAt: "2020-02-02T22:22:22Z"
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: future
spec:
  email: future@example.com
  groups:
  - Admins
  - Everyone
  password: password
  ttl: 60m
status:
  expireAt: "2150-10-10T10:10:10Z"
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: without-ttl
spec:
  email: without-ttl@example.com
  groups:
  - Admins
  - Everyone
  password: password
`)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})

		When("User expired (.status.expireAt < time.Now())", func() {
			It("Should delete user CR", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesGlobalResource("User", "admin").Exists()).Should(BeFalse())
			})
		})

		When("User not expired (.status.expireAt > time.Now())", func() {
			It("Should keep user CR", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesGlobalResource("User", "future").Exists()).Should(BeTrue())
			})
		})

		When("User without ttl", func() {
			It("Should keep user CR", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesGlobalResource("User", "without-ttl").Exists()).Should(BeTrue())
			})
		})
	})
})
