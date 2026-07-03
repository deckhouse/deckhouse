/*
Copyright 2023 Flant JSC

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
	"time"

	. "github.com/benjamintf1/unmarshalledmatchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: get dex user crds ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "User", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Group", false)
	f.RegisterCRD("dex.coreos.com", "v1", "Password", true)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})

		Context("With adding User object", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: password
  ttl: 30m
---
apiVersion: deckhouse.io/v1alpha1
kind: Group
metadata:
  name: gods
spec:
  name: Gods
  members:
  - kind: User
    name: Athena
  - kind: User
    name: Minerva
  - kind: User
    name: Freya
  - kind: User
    name: admin
  - kind: Group
    name: greek
---
apiVersion: deckhouse.io/v1alpha1
kind: Group
metadata:
  name: greek
spec:
  name: Gods
  members:
  - kind: User
    name: Aphrodite
`))
				f.RunHook()
			})
			It("Should fill internal values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchJSON(`
[{
    "name": "admin",
    "spec": {
      "email": "admin@example.com",
      "groups": ["Gods"],
      "password": "password",
      "userID": "admin"
    },
    "encodedName": "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf",
    "status": {
      "lock": {
        "state": false
      }
    }
}]`))

				Expect(
					f.KubernetesGlobalResource("User", "admin").Field("status.expireAt").Time(),
				).Should(
					// TODO: if you specify fakeClock, the test will be more relevant
					BeTemporally("~", time.Now().Add(30*time.Minute), 5*time.Minute),
				)
			})

			When("User resource changed", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: password
  ttl: 1h10m
status:
  expireAt: "2020-02-02T22:22:22Z"
`))
					f.RunHook()
				})

				It("Should not change expire time", func() {
					t, err := time.Parse(time.RFC3339, "2020-02-02T22:22:22Z")
					Expect(f).To(ExecuteSuccessfully())
					Expect(err).ShouldNot(HaveOccurred())
					Expect(
						f.KubernetesGlobalResource("User", "admin").Field("status.expireAt").Time(),
					).Should(
						BeTemporally("==", t),
					)
				})
			})

			Context("With deleting User object", func() {
				BeforeEach(func() {
					f.KubeStateSet("")
					f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
					f.RunHook()
				})
				It("Should delete entry from internal values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

					Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchJSON("[]"))
				})
			})
			Context("With updating User object", func() {
				BeforeEach(func() {
					f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: adminNext@example.com
  password: password
`)
					f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
					f.RunHook()
				})
				It("Should update entry in internal values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

					Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchJSON(`
[{
  "name": "admin",
  "spec": {
    "email": "adminNext@example.com",
    "password": "password",
    "userID": "admin"
  },
  "encodedName": "mfsg22lonzsxq5camv4gc3lqnrss4y3pnxf7fhheqqrcgji",
  "status": {
    "lock": {
      "state": false
    }
  }
}]`))
				})
			})
		})
	})

	Context("Cluster with User object", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: password
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: user
spec:
  email: user@example.com
  password: passwordNext
`))
			f.RunHook()
		})
		It("Should synchronize objects and fill internal values", func() {
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchUnorderedJSON(`
[
  {
    "name": "admin",
    "spec": {
      "email": "admin@example.com",
      "password": "password",
      "userID": "admin"
    },
    "encodedName": "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf",
    "status": {
      "lock": {
        "state": false
      }
    }
  },
  {
    "name": "user",
    "spec": {
      "email": "user@example.com",
      "password": "passwordNext",
      "userID": "user"
    },
    "encodedName": "ovzwk4samv4gc3lqnrss4y3pnxf7fhheqqrcgji",
    "status": {
      "lock": {
        "state": false
      }
    }
  }
]`))
		})
	})

	Context("Cluster with User objects (without groups)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: password
`))
			f.RunHook()
		})
		It("Should synchronize objects and fill internal values and status", func() {
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchUnorderedJSON(`
[
  {
    "name": "admin",
    "spec": {
      "email": "admin@example.com",
      "password": "password",
      "userID": "admin"
    },
    "encodedName": "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf",
    "status": {
      "lock": {
        "state": false
      }
    }
  }
]`))
			Expect(f.KubernetesGlobalResource("User", "admin").Field("status.groups").String()).To(MatchUnorderedJSON(`[]`))
			Expect(f.KubernetesGlobalResource("User", "admin").Field("status.expireAt").Exists()).To(BeFalse())

		})
	})

	Context("Cluster with User and Group objects", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: password
---
apiVersion: deckhouse.io/v1alpha1
kind: Group
metadata:
  name: group-1
spec:
  name: group-1
  members:
  - kind: User
    name: admin
---
apiVersion: deckhouse.io/v1alpha1
kind: Group
metadata:
  name: group-2
spec:
  name: group-2
  members:
  - kind: Group
    name: group-1
---
apiVersion: deckhouse.io/v1alpha1
kind: Group
metadata:
  name: group-3
spec:
  name: group-3
  members:
  - kind: User
    name: admin
`))
			f.RunHook()
		})
		It("Should synchronize objects and fill internal values and status", func() {
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchUnorderedJSON(`
[
  {
    "name": "admin",
    "spec": {
      "email": "admin@example.com",
      "groups": [
        "group-1",
        "group-2",
        "group-3"
      ],
      "password": "password",
      "userID": "admin"
    },
    "encodedName": "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf",
    "status": {
      "lock": {
        "state": false
      }
    }
  }
]`))
			Expect(f.KubernetesGlobalResource("User", "admin").Field("status.groups").String()).To(MatchUnorderedJSON(`["group-1", "group-2", "group-3"]`))
			Expect(f.KubernetesGlobalResource("User", "admin").Field("status.expireAt").Exists()).To(BeFalse())

		})
	})

	Context("One group has been deleted, the user's status should be updated", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: password
status:
  groups:
  - group-1
  - group-2
---
apiVersion: deckhouse.io/v1alpha1
kind: Group
metadata:
  name: group-1
spec:
  name: group-1
  members:
  - kind: User
    name: admin
`))
			f.RunHook()
		})
		It("Should synchronize objects and fill internal values and status", func() {
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchUnorderedJSON(`
[
  {
    "name": "admin",
    "spec": {
      "email": "admin@example.com",
      "groups": [
        "group-1"
      ],
      "password": "password",
      "userID": "admin"
    },
    "encodedName": "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf",
    "status": {
      "lock": {
        "state": false
      }
    }
  }
]`))

			Expect(f.KubernetesGlobalResource("User", "admin").Field("status.groups").String()).To(MatchUnorderedJSON(`["group-1"]`))
			Expect(f.KubernetesGlobalResource("User", "admin").Field("status.expireAt").Exists()).To(BeFalse())

		})

	})

	Context("Cluster with User (with status.groups field filled) and Group objects", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: password
status:
  groups:
  - group-1
  - group-2
---
apiVersion: deckhouse.io/v1alpha1
kind: Group
metadata:
  name: group-1
spec:
  name: group-1
  members:
  - kind: User
    name: admin
`))
			f.RunHook()
		})
		It("Groups in user status should be updated", func() {
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchUnorderedJSON(`
[
  {
    "name": "admin",
    "spec": {
      "email": "admin@example.com",
      "password": "password",
      "groups": [
        "group-1"
      ],
      "userID": "admin"
    },
    "encodedName": "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf",
    "status": {
      "lock": {
        "state": false
      }
    }
  }
]`))
			Expect(f.KubernetesGlobalResource("User", "admin").Field("status.groups").String()).To(MatchUnorderedJSON(`["group-1"]`))
			Expect(f.KubernetesGlobalResource("User", "admin").Field("status.expireAt").Exists()).To(BeFalse())

		})
	})

	Context("Cluster with User (with status.groups field filled)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: password
status:
  groups:
  - group-1
  - group-2
`))
			f.RunHook()
		})
		It("Groups in user status should be updated", func() {
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchUnorderedJSON(`
[
  {
    "name": "admin",
    "spec": {
      "email": "admin@example.com",
      "password": "password",
      "userID": "admin"
    },
    "encodedName": "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf",
    "status": {
      "lock": {
        "state": false
      }
    }
  }
]`))
			Expect(f.KubernetesGlobalResource("User", "admin").Field("status.groups").String()).To(MatchUnorderedJSON(`[]`))
			Expect(f.KubernetesGlobalResource("User", "admin").Field("status.expireAt").Exists()).To(BeFalse())

		})
	})

	Context("Cluster with User having userID set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: password
  userID: myadmin
`))
			f.RunHook()
		})
		It("User's userID field should be overridden", func() {
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchUnorderedJSON(`
[
  {
    "name": "admin",
    "spec": {
      "email": "admin@example.com",
      "password": "password",
      "userID": "admin"
    },
    "encodedName": "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf",
    "status": {
      "lock": {
        "state": false
      }
    }
  }
]`))
		})
	})

	Context("Cluster with local password and linked user", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: password
  userID: myadmin
---
apiVersion: dex.coreos.com/v1
email: admin@example.com
hash: JDJhJDEwJDJiMmNVOENQaE9UYUdyczFIUlF1QXVlUzdKVFQ1WkhzSFN6WWlGUG0xbGVaY2s3TWM4VDRXCg==
hashUpdatedAt: "0001-01-01T00:00:00Z"
incorrectPasswordLoginAttempts: 0
kind: Password
lockedUntil: "2077-01-01T00:00:00Z"
metadata:
  name: mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf
  namespace: d8-user-authn
  labels:
    heritage: deckhouse
    module: user-authn
    app: dex
userID: myadmin
username: admin
`))
			f.RunHook()
		})
		It("User Must sync lock fields with password", func() {
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchUnorderedJSON(`
[
  {
    "name": "admin",
    "spec": {
      "email": "admin@example.com",
      "password": "JDJhJDEwJDJiMmNVOENQaE9UYUdyczFIUlF1QXVlUzdKVFQ1WkhzSFN6WWlGUG0xbGVaY2s3TWM4VDRXCg==",
      "userID": "admin"
    },
    "encodedName": "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf",
    "status": {
      "lock": {
        "state": true,
        "reason": "PasswordPolicyLockout",
        "message": "Locked due to too many failed login attempts",
        "until": "2077-01-01T00:00:00Z"
      }
    }
  }
]`))
		})
	})

	Context("Cluster with local password and linked user with locked state", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: password
  userID: myadmin
  lock:
    message: Locked due to too many failed login attempts
    reason: PasswordPolicyLockout
    state: true
    until: "0001-01-01T00:00:00Z"
---
apiVersion: dex.coreos.com/v1
email: admin@example.com
hash: JDJhJDEwJDJiMmNVOENQaE9UYUdyczFIUlF1QXVlUzdKVFQ1WkhzSFN6WWlGUG0xbGVaY2s3TWM4VDRXCg==
hashUpdatedAt: "0001-01-01T00:00:00Z"
incorrectPasswordLoginAttempts: 0
kind: Password
lockedUntil: "0001-01-01T00:00:00Z"
metadata:
  name: mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf
  namespace: d8-user-authn
  labels:
    heritage: deckhouse
    module: user-authn
    app: dex
userID: myadmin
username: admin
`))
			f.RunHook()
		})
		It("User must sync lock fields with Password", func() {
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchUnorderedJSON(`
[
  {
    "name": "admin",
    "spec": {
      "email": "admin@example.com",
      "password": "JDJhJDEwJDJiMmNVOENQaE9UYUdyczFIUlF1QXVlUzdKVFQ1WkhzSFN6WWlGUG0xbGVaY2s3TWM4VDRXCg==",
      "userID": "admin"
    },
    "encodedName": "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf",
    "status": {
      "lock": {
        "state": false
      }
    }
  }
]`))
		})
	})

	Context("With a new User and no existing Password", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: $2a$10$E/MjyzFi6GZkta9GHd8zCeuYigbLenXv18jkxOZ6vhoWsKnaxNJou
`))
			f.RunHook()
		})
		It("Should create a Password object with hashUpdatedAt stamped to now", func() {
			Expect(f).To(ExecuteSuccessfully())

			password := f.KubernetesResource("Password", "d8-user-authn", "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf")
			Expect(password.Exists()).To(BeTrue())
			Expect(password.Field("email").String()).To(Equal("admin@example.com"))
			Expect(password.Field("username").String()).To(Equal("admin"))
			Expect(password.Field("userID").String()).To(Equal("admin"))
			// A bcrypt hash ($2...) is stored base64-encoded, like the old template did.
			Expect(password.Field("hash").String()).To(Equal("JDJhJDEwJEUvTWp5ekZpNkdaa3RhOUdIZDh6Q2V1WWlnYkxlblh2MThqa3hPWjZ2aG9Xc0tuYXhOSm91"))
			Expect(password.Field("metadata.labels.heritage").String()).To(Equal("deckhouse"))
			Expect(password.Field("metadata.labels.module").String()).To(Equal("user-authn"))
			Expect(password.Field("metadata.labels.app").String()).To(Equal("dex"))
			Expect(password.Field(`metadata.annotations.helm\.sh/resource-policy`).String()).To(Equal("keep"))
			// hashUpdatedAt must be ~now so the rotation policy does not treat a
			// freshly created user as having an ancient (zero-time) password.
			Expect(password.Field("hashUpdatedAt").Time()).To(BeTemporally("~", time.Now(), time.Minute))
		})
	})

	Context("With an existing user-changed Password and a group change", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: $2a$10$thisIsTheOriginalUserSpecHashMustNotOverwriteLiveHashAA
---
apiVersion: deckhouse.io/v1alpha1
kind: Group
metadata:
  name: group-1
spec:
  name: group-1
  members:
  - kind: User
    name: admin
---
apiVersion: dex.coreos.com/v1
email: admin@example.com
hash: dXNlckNoYW5nZWRIYXNo
hashUpdatedAt: "2024-01-02T03:04:05Z"
incorrectPasswordLoginAttempts: 3
lockedUntil: "2077-01-01T00:00:00Z"
previousHashes:
- b2xkSGFzaA==
requireResetHashOnNextSuccLogin: true
kind: Password
metadata:
  name: mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf
  namespace: d8-user-authn
  labels:
    heritage: deckhouse
    module: user-authn
    app: dex
  annotations:
    helm.sh/resource-policy: keep
userID: admin
username: admin
`))
			f.RunHook()
		})
		It("Should update the owned fields without clobbering Dex-managed runtime fields", func() {
			Expect(f).To(ExecuteSuccessfully())

			password := f.KubernetesResource("Password", "d8-user-authn", "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf")
			Expect(password.Exists()).To(BeTrue())
			// Deckhouse-owned field is reconciled.
			Expect(password.Field("groups").String()).To(MatchJSON(`["group-1"]`))
			// Dex-managed runtime fields are left untouched.
			Expect(password.Field("hash").String()).To(Equal("dXNlckNoYW5nZWRIYXNo"))
			Expect(password.Field("hashUpdatedAt").String()).To(Equal("2024-01-02T03:04:05Z"))
			Expect(password.Field("incorrectPasswordLoginAttempts").Int()).To(Equal(int64(3)))
			Expect(password.Field("lockedUntil").String()).To(Equal("2077-01-01T00:00:00Z"))
			Expect(password.Field("requireResetHashOnNextSuccLogin").Bool()).To(BeTrue())
			Expect(password.Field("previousHashes").String()).To(MatchJSON(`["b2xkSGFzaA=="]`))
		})
	})

	Context("With orphaned Password objects", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: dex.coreos.com/v1
email: ghost@example.com
hash: aGFzaA==
kind: Password
metadata:
  name: managed-ghost
  namespace: d8-user-authn
  labels:
    heritage: deckhouse
    module: user-authn
    app: dex
userID: ghost
username: ghost
---
apiVersion: dex.coreos.com/v1
email: external@example.com
hash: aGFzaA==
kind: Password
metadata:
  name: unmanaged-ghost
  namespace: d8-user-authn
userID: external
username: external
`))
			f.RunHook()
		})
		It("Should delete the module-managed orphan and keep the unmanaged one", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Password", "d8-user-authn", "managed-ghost").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Password", "d8-user-authn", "unmanaged-ghost").Exists()).To(BeTrue())
		})
	})

	Context("With a User whose email changed (Password object renamed)", func() {
		BeforeEach(func() {
			// The previous email hashed to a different object name. After the email
			// change the old object is no longer referenced by that name, but the
			// user still owns it (matched by username). The live hash below differs
			// from User.spec.password on purpose: it represents a password the user
			// rotated after account creation, which must survive the rename instead
			// of being reverted to the immutable creation-time spec.password.
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: $2a$10$thisIsTheOriginalUserSpecHashMustNotOverwriteLiveHashAA
---
apiVersion: dex.coreos.com/v1
email: old@example.com
hash: dXNlckNoYW5nZWRIYXNo
hashUpdatedAt: "2024-01-02T03:04:05Z"
incorrectPasswordLoginAttempts: 3
lockedUntil: "2077-01-01T00:00:00Z"
previousHashes:
- b2xkSGFzaA==
requireResetHashOnNextSuccLogin: true
kind: Password
metadata:
  name: oldemailencodedname
  namespace: d8-user-authn
  labels:
    heritage: deckhouse
    module: user-authn
    app: dex
userID: admin
username: admin
`))
			f.RunHook()
		})
		It("Should delete the old-name Password and recreate it under the new email encoding, preserving the live hash", func() {
			Expect(f).To(ExecuteSuccessfully())
			// Object keyed by the previous email encoding is orphaned and removed.
			Expect(f.KubernetesResource("Password", "d8-user-authn", "oldemailencodedname").Exists()).To(BeFalse())
			// A fresh object appears under the new email's Dex (FNV-like) encoding.
			newPassword := f.KubernetesResource("Password", "d8-user-authn", "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf")
			Expect(newPassword.Exists()).To(BeTrue())
			Expect(newPassword.Field("email").String()).To(Equal("admin@example.com"))
			// The live hash the user last set is carried over, NOT reseeded from
			// the CR's original creation-time password.
			Expect(newPassword.Field("hash").String()).To(Equal("dXNlckNoYW5nZWRIYXNo"))
			// Other Dex-managed runtime state is carried over as well.
			Expect(newPassword.Field("hashUpdatedAt").String()).To(Equal("2024-01-02T03:04:05Z"))
			Expect(newPassword.Field("incorrectPasswordLoginAttempts").Int()).To(Equal(int64(3)))
			Expect(newPassword.Field("lockedUntil").String()).To(Equal("2077-01-01T00:00:00Z"))
			Expect(newPassword.Field("requireResetHashOnNextSuccLogin").Bool()).To(BeTrue())
			Expect(newPassword.Field("previousHashes").String()).To(MatchJSON(`["b2xkSGFzaA=="]`))
		})
	})

	Context("With a User whose TTL is out of range", func() {
		BeforeEach(func() {
			// 9999999h overflows time.ParseDuration. Such a value would normally be
			// rejected by the CRD pattern, but the hook must never hard-fail on it:
			// returning an error would retry forever and block the module queue.
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: $2a$10$E/MjyzFi6GZkta9GHd8zCeuYigbLenXv18jkxOZ6vhoWsKnaxNJou
  ttl: 9999999h
`))
			f.RunHook()
		})
		It("Should not fail the hook and still reconcile the Password", func() {
			Expect(f).To(ExecuteSuccessfully())
			password := f.KubernetesResource("Password", "d8-user-authn", "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf")
			Expect(password.Exists()).To(BeTrue())
			// No expiry is recorded for the broken TTL.
			Expect(f.KubernetesResource("User", "", "admin").Field("status.expireAt").String()).To(Equal(""))
		})
	})

})
