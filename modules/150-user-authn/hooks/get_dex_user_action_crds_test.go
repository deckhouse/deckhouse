/*
Copyright 2021 Flant JSC

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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: handle UserAction creation ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")

	f.RegisterCRD("deckhouse.io", "v1", "UserAction", false)
	f.RegisterCRD("dex.coreos.com", "v1", "Password", true)
	f.RegisterCRD("dex.coreos.com", "v1", "OfflineSession", true)

	nowStr := time.Now().UTC().Format(time.RFC3339)
	const (
		password = `
---
apiVersion: dex.coreos.com/v1
email: admin@yourcompany.com
hash: JDJhJDEwJDlFRXFCMFNlenkyZk1ZT2JIZU1tUHVHSHo2bElZV1FCRTAxY3pYZFVmOUs5NlFJVlpVQlF1
hashUpdatedAt: "2025-09-24T04:33:04.493729966Z"
incorrectPasswordLoginAttempts: 0
kind: Password
lockedUntil: null
metadata:
  creationTimestamp: "%s"
  name: mfsg22loib4w65lsmnxw24dbnz4s4y3pnxf7fhheqqrcgji
  namespace: d8-user-authn
previousHashes:
- JDJ5JDEwJGRNWGVGUVBkdUdYYVMyWDFPcGdZdk9HSy81LkdsNm5sdU9mUkhnNWlQdDhuSlh6SzhpeS5H
- JDJhJDEwJHNXR09ZSjBvSjZLWnlGMjJjZUJ2cXVEYnhycktQd2FOQUtjVjZjL0hwMGd3Y2h5dmRWWmZT
userID: admin
username: admin
`
		passwordLocked = `
---
apiVersion: dex.coreos.com/v1
email: admin@yourcompany.com
hash: JDJhJDEwJDlFRXFCMFNlenkyZk1ZT2JIZU1tUHVHSHo2bElZV1FCRTAxY3pYZFVmOUs5NlFJVlpVQlF1
hashUpdatedAt: "2025-09-24T04:33:04.493729966Z"
incorrectPasswordLoginAttempts: 0
kind: Password
lockedUntil: '2077-07-12T00:00:00Z'
metadata:
  creationTimestamp: "%s"
  name: mfsg22loib4w65lsmnxw24dbnz4s4y3pnxf7fhheqqrcgji
  namespace: d8-user-authn
previousHashes:
- JDJ5JDEwJGRNWGVGUVBkdUdYYVMyWDFPcGdZdk9HSy81LkdsNm5sdU9mUkhnNWlQdDhuSlh6SzhpeS5H
- JDJhJDEwJHNXR09ZSjBvSjZLWnlGMjJjZUJ2cXVEYnhycktQd2FOQUtjVjZjL0hwMGd3Y2h5dmRWWmZT
userID: admin
username: admin
`
		userActionLock = `
---
apiVersion: deckhouse.io/v1
kind: UserAction
metadata:
  creationTimestamp: "%s"
  name: user-action-01
spec:
  initiatorType: Admin
  lock:
    for: 1h
  type: Lock
  user: admin
`
		userActionInvalidLock = `
---
apiVersion: deckhouse.io/v1
kind: UserAction
metadata:
  creationTimestamp: "%s"
  name: user-action-01
spec:
  initiatorType: Admin
  type: Lock
  user: admin
`
		userActionUnlock = `
---
apiVersion: deckhouse.io/v1
kind: UserAction
metadata:
  creationTimestamp: "%s"
  name: user-action-01
spec:
  initiatorType: Admin
  type: Unlock
  user: admin
`
		userActionResetPassword = `
---
apiVersion: deckhouse.io/v1
kind: UserAction
metadata:
  creationTimestamp: "%s"
  name: user-action-01
spec:
  initiatorType: Admin
  type: ResetPassword
  resetPassword:
    newPasswordHash: '$newHash'
  user: admin
`
		userActionInvalidResetPassword = `
---
apiVersion: deckhouse.io/v1
kind: UserAction
metadata:
  creationTimestamp: "%s"
  name: user-action-01
spec:
  initiatorType: Admin
  type: ResetPassword
  user: admin
`
		userActionReset2FA = `
---
apiVersion: deckhouse.io/v1
kind: UserAction
metadata:
  creationTimestamp: "%s"
  name: user-action-01
spec:
  initiatorType: Admin
  type: Reset2FA
  user: admin
`
		offlineSessions = `
---
apiVersion: dex.coreos.com/v1
kind: OfflineSession
metadata:
  creationTimestamp: "%s"
  name: offsess-1
  namespace: d8-user-authn
userID: admin
connID: abcde
refresh: {}
connectorData: DdasiFSk/asd1
totp: abcdexx
totpConfirmed: true
---
apiVersion: dex.coreos.com/v1
kind: OfflineSession
metadata:
  creationTimestamp: "%s"
  name: offsess-2
  namespace: d8-user-authn
userID: admin
connID: abcde2
refresh: {}
connectorData: DdasiFSk/asd2
totp: abcdexx
totpConfirmed: true
`
		oldUserActions = `
---
apiVersion: deckhouse.io/v1
kind: UserAction
metadata:
  creationTimestamp: "%s"
  name: old-user-action-1
spec:
  initiatorType: Admin
  lock:
    for: 1h
  type: Lock
  user: admin
status:
  completedAt: "2025-09-23T19:39:13Z"
  phase: Succeeded
---
apiVersion: deckhouse.io/v1
kind: UserAction
metadata:
  creationTimestamp: "%s"
  name: old-user-action-2
spec:
  initiatorType: Admin
  lock:
    for: 1h
  type: Unlock
  user: admin
status:
  completedAt: "2025-09-24T19:40:13Z"
  phase: Succeeded
`
	)

	Context("Cluster with existing User and Password :: Processing new UserAction with success cases", func() {
		It("Lock local user", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(password, nowStr) + fmt.Sprintf(userActionLock, nowStr),
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			pw := f.KubernetesResource("Password", "d8-user-authn", "mfsg22loib4w65lsmnxw24dbnz4s4y3pnxf7fhheqqrcgji")
			Expect(pw.Field("lockedUntil").Time()).To(BeTemporally("~", time.Now().Add(1*time.Hour), 5*time.Second))
			Expect(pw.Field("metadata.annotations").Map()).To(HaveKey("deckhouse.io/locked-by-administrator"))

			ua := f.KubernetesGlobalResource("UserAction", "user-action-01")
			Expect(ua.Field("status.phase").String()).To(Equal("Succeeded"))
			Expect(ua.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Unlock local user", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(passwordLocked, nowStr) + fmt.Sprintf(userActionUnlock, nowStr),
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			pw := f.KubernetesResource("Password", "d8-user-authn", "mfsg22loib4w65lsmnxw24dbnz4s4y3pnxf7fhheqqrcgji")
			Expect(pw.Field("lockedUntil").Time().IsZero()).To(BeTrue())

			ua := f.KubernetesGlobalResource("UserAction", "user-action-01")
			Expect(ua.Field("status.phase").String()).To(Equal("Succeeded"))
			Expect(ua.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Reset user's password", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(password, nowStr) + fmt.Sprintf(userActionResetPassword, nowStr),
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			pw := f.KubernetesResource("Password", "d8-user-authn", "mfsg22loib4w65lsmnxw24dbnz4s4y3pnxf7fhheqqrcgji")
			// JG5ld0hhc2g= - base64 encoded `$newHash` string
			Expect(pw.Field("hash").String()).To(Equal("JG5ld0hhc2g="))
			Expect(pw.Field("requireResetHashOnNextSuccLogin").Bool()).To(BeTrue())

			ua := f.KubernetesGlobalResource("UserAction", "user-action-01")
			Expect(ua.Field("status.phase").String()).To(Equal("Succeeded"))
			Expect(ua.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Reset user's 2FA", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(offlineSessions, nowStr, nowStr) + fmt.Sprintf(userActionReset2FA, nowStr),
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			for _, offsessName := range []string{"offsess-1", "offsess-2"} {
				offsess := f.KubernetesResource("OfflineSessions", "d8-user-authn", offsessName)
				Expect(offsess.Exists()).To(BeFalse())
			}

			ua := f.KubernetesGlobalResource("UserAction", "user-action-01")
			Expect(ua.Field("status.phase").String()).To(Equal("Succeeded"))
			Expect(ua.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Clean up old userActions", func() {
			dayAgoStr := time.Now().Add(-25 * time.Hour).UTC().Format(time.RFC3339)

			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(oldUserActions, dayAgoStr, dayAgoStr),
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			for _, uaName := range []string{"old-user-action-1", "old-user-action-2"} {
				ua := f.KubernetesGlobalResource("UserAction", uaName)
				Expect(ua.Exists()).To(BeFalse())
			}
		})
	})

	Context("Cluster with existing User and Password :: Processing new UserAction with fail cases", func() {
		It("Lock local user with insuffisent userAction's fields", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(password, nowStr) + fmt.Sprintf(userActionInvalidLock, nowStr),
			))
			f.RunHook()

			ua := f.KubernetesGlobalResource("UserAction", "user-action-01")
			Expect(ua.Field("status.phase").String()).To(Equal("Failed"))
			Expect(ua.Field("status.message").String()).NotTo(BeEmpty())
			Expect(ua.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Lock local user w/o password entity", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(userActionLock, nowStr),
			))
			f.RunHook()

			ua := f.KubernetesGlobalResource("UserAction", "user-action-01")
			Expect(ua.Field("status.phase").String()).To(Equal("Failed"))
			Expect(ua.Field("status.message").String()).NotTo(BeEmpty())
			Expect(ua.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Unlock local user w/o password entity", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(userActionUnlock, nowStr),
			))
			f.RunHook()

			ua := f.KubernetesGlobalResource("UserAction", "user-action-01")
			Expect(ua.Field("status.phase").String()).To(Equal("Failed"))
			Expect(ua.Field("status.message").String()).NotTo(BeEmpty())
			Expect(ua.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Reset user's password with insuffisent userAction's fields", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(password, nowStr) + fmt.Sprintf(userActionInvalidResetPassword, nowStr),
			))
			f.RunHook()

			ua := f.KubernetesGlobalResource("UserAction", "user-action-01")
			Expect(ua.Field("status.phase").String()).To(Equal("Failed"))
			Expect(ua.Field("status.message").String()).NotTo(BeEmpty())
			Expect(ua.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Reset user's password w/o password entity", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(userActionResetPassword, nowStr),
			))
			f.RunHook()

			ua := f.KubernetesGlobalResource("UserAction", "user-action-01")
			Expect(ua.Field("status.phase").String()).To(Equal("Failed"))
			Expect(ua.Field("status.message").String()).NotTo(BeEmpty())
			Expect(ua.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Reset user's 2FA w/o offlinesessions entity", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(userActionReset2FA, nowStr),
			))
			f.RunHook()

			ua := f.KubernetesGlobalResource("UserAction", "user-action-01")
			Expect(ua.Field("status.phase").String()).To(Equal("Failed"))
			Expect(ua.Field("status.message").String()).NotTo(BeEmpty())
			Expect(ua.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})
	})
})
