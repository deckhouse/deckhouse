package hooks

import (
	. "github.com/benjamintf1/unmarshalledmatchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Node user hooks :: get nodeuser crds ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "nodeUser", false)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})

		Context("With adding nodeUser object", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: NodeUser
metadata:
  name: test.user
spec:
  uid: 1001
  sshPublicKey: "ssh-rsa testsshkey"
  passwordHash: "$saltpasswordhash"
  isSudoer: true
  extraGroups:
  - extragroup
`))
				f.RunHook()
			})
			It("Should fill internal values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("nodeManager.internal.nodeUsers").String()).To(MatchJSON(`
[{
    "name": "test.user",
    "spec": {
      "uid":1001,
      "sshPublicKey": "ssh-rsa testsshkey",
      "passwordHash": "$saltpasswordhash",
      "isSudoer": true,
      "extraGroups": ["extragroup"]
    }
}]`))
			})

			Context("With deleting nodeUser object", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(""))
					f.RunHook()
				})
				It("Should delete entry from internal values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

					Expect(f.ValuesGet("nodeManager.internal.nodeUsers").String()).To(MatchJSON("[]"))
				})
			})
			Context("With updating nodeUser object", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: NodeUser
metadata:
  name: test.user
spec:
  uid: 1001
  sshPublicKey: "ssh-rsa testsshkey2"
  passwordHash: "$saltpasswordhash2"
  isSudoer: false
  extraGroups:
  - extragroup
  - extragroup2
`))
					f.RunHook()
				})
				It("Should update entry in internal values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

					Expect(f.ValuesGet("nodeManager.internal.nodeUsers").String()).To(MatchJSON(`
[{
    "name": "test.user",
    "spec": {
      "uid": 1001,
      "sshPublicKey": "ssh-rsa testsshkey2",
      "passwordHash": "$saltpasswordhash2",
      "isSudoer": false,
      "extraGroups": ["extragroup", "extragroup2"]
    }
}]`))
				})
			})
		})
	})

	Context("Many nodeUser objects", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeUser
metadata:
  name: test.user1
spec:
  uid: 1001
  sshPublicKey: "ssh-rsa testsshkey"
  passwordHash: "$saltpasswordhash"
  isSudoer: true
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeUser
metadata:
  name: test.user2
spec:
  uid: 1002
  sshPublicKey: "ssh-rsa testsshkey2"
  passwordHash: "$saltpasswordhash2"
  isSudoer: false
  extraGroups:
  - extragroup
`))
			f.RunHook()
		})
		It("Should synchronize objects and fill internal values", func() {
			Expect(f.ValuesGet("nodeManager.internal.nodeUsers").String()).To(MatchUnorderedJSON(`
[
  {
    "name": "test.user1",
    "spec": {
      "uid": 1001,
      "sshPublicKey": "ssh-rsa testsshkey",
      "passwordHash": "$saltpasswordhash",
      "isSudoer": true
    }
  },
  {
    "name": "test.user2",
    "spec": {
      "uid": 1002,
      "sshPublicKey": "ssh-rsa testsshkey2",
      "passwordHash": "$saltpasswordhash2",
      "isSudoer": false,
      "extraGroups": ["extragroup"]
    }
  }
]`))
		})
	})

	Context("Many nodeUser objects with same UIDs", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeUser
metadata:
  name: test.user1
spec:
  uid: 1001
  sshPublicKey: "ssh-rsa testsshkey"
  passwordHash: "$saltpasswordhash"
  isSudoer: true
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeUser
metadata:
  name: test.user2
spec:
  uid: 1001
  sshPublicKey: "ssh-rsa testsshkey2"
  passwordHash: "$saltpasswordhash2"
  isSudoer: false
  extraGroups:
  - extragroup
`))
			f.RunHook()
		})
		It("Hook must fail", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: UIDs are not unique among NodeUser CRs.`))
		})
	})

})
