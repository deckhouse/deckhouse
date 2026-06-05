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
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: handle UserOperation creation ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")

	f.RegisterCRD("deckhouse.io", "v1", "UserOperation", false)
	f.RegisterCRD("dex.coreos.com", "v1", "Password", true)
	f.RegisterCRD("dex.coreos.com", "v1", "OfflineSessions", true)
	f.RegisterCRD("dex.coreos.com", "v1", "RefreshToken", true)

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
  annotations:
    deckhouse.io/locked-by-administrator: ""
  creationTimestamp: "%s"
  name: mfsg22loib4w65lsmnxw24dbnz4s4y3pnxf7fhheqqrcgji
  namespace: d8-user-authn
previousHashes:
- JDJ5JDEwJGRNWGVGUVBkdUdYYVMyWDFPcGdZdk9HSy81LkdsNm5sdU9mUkhnNWlQdDhuSlh6SzhpeS5H
- JDJhJDEwJHNXR09ZSjBvSjZLWnlGMjJjZUJ2cXVEYnhycktQd2FOQUtjVjZjL0hwMGd3Y2h5dmRWWmZT
userID: admin
username: admin
`
		userOperationLock = `
---
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  creationTimestamp: "%s"
  name: user-operation-01
spec:
  initiatorType: Admin
  lock:
    for: 1h
  type: Lock
  user: admin
`
		userOperationInvalidLock = `
---
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  creationTimestamp: "%s"
  name: user-operation-01
spec:
  initiatorType: Admin
  type: Lock
  user: admin
`
		userOperationLockPermanent = `
---
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  creationTimestamp: "%s"
  name: user-operation-01
spec:
  initiatorType: Admin
  lock:
    for: permanent
  type: Lock
  user: admin
`
		userOperationLockSevenDays = `
---
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  creationTimestamp: "%s"
  name: user-operation-01
spec:
  initiatorType: Admin
  lock:
    for: 7d
  type: Lock
  user: admin
`
		userOperationUnlock = `
---
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  creationTimestamp: "%s"
  name: user-operation-01
spec:
  initiatorType: Admin
  type: Unlock
  user: admin
`
		userOperationResetPassword = `
---
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  creationTimestamp: "%s"
  name: user-operation-01
spec:
  initiatorType: Admin
  type: ResetPassword
  resetPassword:
    newPasswordHash: '$2y$10$9fdmv4ewdvzVCTQ01BnAZ.Cy27fdnfNkl.dLIge2YS2gSF4czqXUy'
  user: admin
`
		userOperationInvalidResetPassword = `
---
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  creationTimestamp: "%s"
  name: user-operation-01
spec:
  initiatorType: Admin
  type: ResetPassword
  user: admin
`
		userOperationReset2FA = `
---
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  creationTimestamp: "%s"
  name: user-operation-01
spec:
  initiatorType: Admin
  type: Reset2FA
  user: admin
`
		userOperationReset2FAExternalTarget = `
---
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  creationTimestamp: "%s"
  name: user-operation-01
spec:
  initiatorType: Admin
  type: Reset2FA
  target:
    connectorID: my-ldap
    email: jane.doe@example.org
`
		refreshTokensForAdmin = `
---
apiVersion: dex.coreos.com/v1
kind: RefreshToken
metadata:
  name: rt-1
  namespace: d8-user-authn
claims:
  email: admin@yourcompany.com
  username: admin
  userID: ""
clientID: console-d8-console-dex-authenticator
connectorID: local
scopes: ["openid", "profile", "email", "offline_access"]
token: token1
---
apiVersion: dex.coreos.com/v1
kind: RefreshToken
metadata:
  name: rt-2
  namespace: d8-user-authn
claims:
  email: admin@yourcompany.com
  username: admin
  userID: ""
clientID: console-d8-console-dex-authenticator
connectorID: local
scopes: ["openid", "profile", "email", "offline_access"]
token: token2
`
		offlineSessionsNoUserID = `
---
apiVersion: dex.coreos.com/v1
kind: OfflineSessions
metadata:
  creationTimestamp: "%s"
  name: offsess-no-userid
  namespace: d8-user-authn
connID: local
refresh:
  console-d8-console-dex-authenticator:
    ClientID: console-d8-console-dex-authenticator
    CreatedAt: "2026-01-18T23:25:02Z"
    ID: rt-1
    LastUsed: "2026-01-18T23:25:02Z"
`
		offlineSessions = `
---
apiVersion: dex.coreos.com/v1
kind: OfflineSessions
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
kind: OfflineSessions
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
		oldUserOperations = `
---
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  creationTimestamp: "%s"
  name: old-user-operation-1
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
kind: UserOperation
metadata:
  creationTimestamp: "%s"
  name: old-user-operation-2
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

	Context("Cluster with existing User and Password :: Processing new UserOperation with success cases", func() {
		It("Lock local user", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(password, nowStr) + fmt.Sprintf(userOperationLock, nowStr),
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			pw := f.KubernetesResource("Password", "d8-user-authn", "mfsg22loib4w65lsmnxw24dbnz4s4y3pnxf7fhheqqrcgji")
			Expect(pw.Field("lockedUntil").Time()).To(BeTemporally("~", time.Now().Add(1*time.Hour), 5*time.Second))
			Expect(pw.Field("metadata.annotations").Map()).To(HaveKey("deckhouse.io/locked-by-administrator"))

			uo := f.KubernetesGlobalResource("UserOperation", "user-operation-01")
			Expect(uo.Field("status.phase").String()).To(Equal("Succeeded"))
			Expect(uo.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Lock local user permanently", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(password, nowStr) + fmt.Sprintf(userOperationLockPermanent, nowStr),
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			pw := f.KubernetesResource("Password", "d8-user-authn", "mfsg22loib4w65lsmnxw24dbnz4s4y3pnxf7fhheqqrcgji")
			// The hook writes the year-9999 sentinel for `for: permanent`.
			// Any year past 9000 implies the permanent-lock branch ran —
			// no finite duration can reach that horizon.
			Expect(pw.Field("lockedUntil").Time().Year()).To(BeNumerically(">=", 9000))
			Expect(pw.Field("metadata.annotations").Map()).To(HaveKey("deckhouse.io/locked-by-administrator"))

			uo := f.KubernetesGlobalResource("UserOperation", "user-operation-01")
			Expect(uo.Field("status.phase").String()).To(Equal("Succeeded"))
			Expect(uo.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Lock local user for 7 days (d unit is expanded to hours)", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(password, nowStr) + fmt.Sprintf(userOperationLockSevenDays, nowStr),
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			pw := f.KubernetesResource("Password", "d8-user-authn", "mfsg22loib4w65lsmnxw24dbnz4s4y3pnxf7fhheqqrcgji")
			// 7 days expansion happens in the hook (Go's time.ParseDuration knows
			// no "d" unit). Verify lockedUntil sits ~7 days from now (±1 minute
			// to absorb scheduling jitter without making the check vacuous).
			Expect(pw.Field("lockedUntil").Time()).To(BeTemporally("~", time.Now().Add(7*24*time.Hour), time.Minute))
			Expect(pw.Field("metadata.annotations").Map()).To(HaveKey("deckhouse.io/locked-by-administrator"))

			uo := f.KubernetesGlobalResource("UserOperation", "user-operation-01")
			Expect(uo.Field("status.phase").String()).To(Equal("Succeeded"))
			Expect(uo.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Reset user's password wipes the hash from spec on success", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(password, nowStr) + fmt.Sprintf(userOperationResetPassword, nowStr),
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			uo := f.KubernetesGlobalResource("UserOperation", "user-operation-01")
			Expect(uo.Field("status.phase").String()).To(Equal("Succeeded"))
			// spec.resetPassword must be removed so the bcrypt hash does not
			// linger in etcd for the 24h retention window after the password
			// has already been applied to the Dex Password CR.
			Expect(uo.Field("spec.resetPassword").Exists()).To(BeFalse())
		})

		It("Unlock local user", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(passwordLocked, nowStr) + fmt.Sprintf(userOperationUnlock, nowStr),
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			pw := f.KubernetesResource("Password", "d8-user-authn", "mfsg22loib4w65lsmnxw24dbnz4s4y3pnxf7fhheqqrcgji")
			Expect(pw.Field("lockedUntil").Time().IsZero()).To(BeTrue())
			Expect(pw.Field("metadata.annotations").Map()).NotTo(HaveKey("deckhouse.io/locked-by-administrator"))

			uo := f.KubernetesGlobalResource("UserOperation", "user-operation-01")
			Expect(uo.Field("status.phase").String()).To(Equal("Succeeded"))
			Expect(uo.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Reset user's password", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(password, nowStr) + fmt.Sprintf(userOperationResetPassword, nowStr),
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			pw := f.KubernetesResource("Password", "d8-user-authn", "mfsg22loib4w65lsmnxw24dbnz4s4y3pnxf7fhheqqrcgji")
			// base64 encoded bcrypt hash from userOperationResetPassword.newPasswordHash
			Expect(pw.Field("hash").String()).To(Equal("JDJ5JDEwJDlmZG12NGV3ZHZ6VkNUUTAxQm5BWi5DeTI3ZmRuZk5rbC5kTElnZTJZUzJnU0Y0Y3pxWFV5"))
			Expect(pw.Field("requireResetHashOnNextSuccLogin").Bool()).To(BeTrue())

			uo := f.KubernetesGlobalResource("UserOperation", "user-operation-01")
			Expect(uo.Field("status.phase").String()).To(Equal("Succeeded"))
			Expect(uo.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Lock local user terminates active sessions", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(password, nowStr) +
					fmt.Sprintf(offlineSessions, nowStr, nowStr) +
					refreshTokensForAdmin +
					fmt.Sprintf(userOperationLock, nowStr),
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			pw := f.KubernetesResource("Password", "d8-user-authn", "mfsg22loib4w65lsmnxw24dbnz4s4y3pnxf7fhheqqrcgji")
			Expect(pw.Field("lockedUntil").Time()).To(BeTemporally("~", time.Now().Add(1*time.Hour), 5*time.Second))
			Expect(pw.Field("metadata.annotations").Map()).To(HaveKey("deckhouse.io/locked-by-administrator"))

			for _, name := range []string{"offsess-1", "offsess-2"} {
				offsess := f.KubernetesResource("OfflineSessions", "d8-user-authn", name)
				Expect(offsess.Exists()).To(BeFalse(), "OfflineSessions %s must be deleted on Lock", name)
			}
			for _, name := range []string{"rt-1", "rt-2"} {
				rt := f.KubernetesResource("RefreshToken", "d8-user-authn", name)
				Expect(rt.Exists()).To(BeFalse(), "RefreshToken %s must be deleted on Lock", name)
			}

			uo := f.KubernetesGlobalResource("UserOperation", "user-operation-01")
			Expect(uo.Field("status.phase").String()).To(Equal("Succeeded"))
			Expect(uo.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Reset user's password terminates active sessions", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(password, nowStr) +
					fmt.Sprintf(offlineSessions, nowStr, nowStr) +
					refreshTokensForAdmin +
					fmt.Sprintf(userOperationResetPassword, nowStr),
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			pw := f.KubernetesResource("Password", "d8-user-authn", "mfsg22loib4w65lsmnxw24dbnz4s4y3pnxf7fhheqqrcgji")
			Expect(pw.Field("hash").String()).To(Equal("JDJ5JDEwJDlmZG12NGV3ZHZ6VkNUUTAxQm5BWi5DeTI3ZmRuZk5rbC5kTElnZTJZUzJnU0Y0Y3pxWFV5"))
			Expect(pw.Field("requireResetHashOnNextSuccLogin").Bool()).To(BeTrue())

			for _, name := range []string{"offsess-1", "offsess-2"} {
				offsess := f.KubernetesResource("OfflineSessions", "d8-user-authn", name)
				Expect(offsess.Exists()).To(BeFalse(), "OfflineSessions %s must be deleted on ResetPassword", name)
			}
			for _, name := range []string{"rt-1", "rt-2"} {
				rt := f.KubernetesResource("RefreshToken", "d8-user-authn", name)
				Expect(rt.Exists()).To(BeFalse(), "RefreshToken %s must be deleted on ResetPassword", name)
			}

			uo := f.KubernetesGlobalResource("UserOperation", "user-operation-01")
			Expect(uo.Field("status.phase").String()).To(Equal("Succeeded"))
			Expect(uo.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Reset user's 2FA", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(offlineSessions, nowStr, nowStr) + fmt.Sprintf(userOperationReset2FA, nowStr),
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			for _, offsessName := range []string{"offsess-1", "offsess-2"} {
				offsess := f.KubernetesResource("OfflineSessions", "d8-user-authn", offsessName)
				Expect(offsess.Exists()).To(BeFalse())
			}

			uo := f.KubernetesGlobalResource("UserOperation", "user-operation-01")
			Expect(uo.Field("status.phase").String()).To(Equal("Succeeded"))
			Expect(uo.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Reset user's 2FA when OfflineSessions has no userID (match via RefreshToken claims)", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(offlineSessionsNoUserID, nowStr) + refreshTokensForAdmin + fmt.Sprintf(userOperationReset2FA, nowStr),
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			offsess := f.KubernetesResource("OfflineSessions", "d8-user-authn", "offsess-no-userid")
			Expect(offsess.Exists()).To(BeFalse())

			for _, rtName := range []string{"rt-1", "rt-2"} {
				rt := f.KubernetesResource("RefreshToken", "d8-user-authn", rtName)
				Expect(rt.Exists()).To(BeFalse())
			}

			uo := f.KubernetesGlobalResource("UserOperation", "user-operation-01")
			Expect(uo.Field("status.phase").String()).To(Equal("Succeeded"))
			Expect(uo.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Reset user's 2FA is idempotent (no objects to delete)", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(userOperationReset2FA, nowStr),
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			uo := f.KubernetesGlobalResource("UserOperation", "user-operation-01")
			Expect(uo.Field("status.phase").String()).To(Equal("Succeeded"))
			Expect(uo.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Clean up old userOperations", func() {
			dayAgoStr := time.Now().Add(-25 * time.Hour).UTC().Format(time.RFC3339)

			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(oldUserOperations, dayAgoStr, dayAgoStr),
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			for _, uoName := range []string{"old-user-operation-1", "old-user-operation-2"} {
				uo := f.KubernetesGlobalResource("UserOperation", uoName)
				Expect(uo.Exists()).To(BeFalse())
			}
		})
	})

	Context("Cluster with existing User and Password :: Processing new UserOperation with fail cases", func() {
		It("Lock local user with insuffisent userOperation's fields", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(password, nowStr) + fmt.Sprintf(userOperationInvalidLock, nowStr),
			))
			f.RunHook()

			uo := f.KubernetesGlobalResource("UserOperation", "user-operation-01")
			Expect(uo.Field("status.phase").String()).To(Equal("Failed"))
			Expect(uo.Field("status.message").String()).NotTo(BeEmpty())
			Expect(uo.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Lock local user w/o password entity", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(userOperationLock, nowStr),
			))
			f.RunHook()

			uo := f.KubernetesGlobalResource("UserOperation", "user-operation-01")
			Expect(uo.Field("status.phase").String()).To(Equal("Failed"))
			Expect(uo.Field("status.message").String()).NotTo(BeEmpty())
			Expect(uo.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Unlock local user w/o password entity", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(userOperationUnlock, nowStr),
			))
			f.RunHook()

			uo := f.KubernetesGlobalResource("UserOperation", "user-operation-01")
			Expect(uo.Field("status.phase").String()).To(Equal("Failed"))
			Expect(uo.Field("status.message").String()).NotTo(BeEmpty())
			Expect(uo.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Reset user's password with insuffisent userOperation's fields", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(password, nowStr) + fmt.Sprintf(userOperationInvalidResetPassword, nowStr),
			))
			f.RunHook()

			uo := f.KubernetesGlobalResource("UserOperation", "user-operation-01")
			Expect(uo.Field("status.phase").String()).To(Equal("Failed"))
			Expect(uo.Field("status.message").String()).NotTo(BeEmpty())
			Expect(uo.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Reset user's password w/o password entity", func() {
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(userOperationResetPassword, nowStr),
			))
			f.RunHook()

			uo := f.KubernetesGlobalResource("UserOperation", "user-operation-01")
			Expect(uo.Field("status.phase").String()).To(Equal("Failed"))
			Expect(uo.Field("status.message").String()).NotTo(BeEmpty())
			Expect(uo.Field("status.completedAt").Time()).To(BeTemporally("~", time.Now(), 5*time.Second))
		})

		It("Reset2FA against an external target fails (local-only operation)", func() {
			// CRD CEL forbids target on Reset2FA; this asserts the hook-side
			// safety net so a target (and the resulting empty spec.user) can
			// never reach invalidateLocalUserSessions and match foreign sessions.
			f.BindingContexts.Set(f.KubeStateSet(
				fmt.Sprintf(userOperationReset2FAExternalTarget, nowStr) +
					fmt.Sprintf(offlineSessions, nowStr, nowStr) +
					refreshTokensForAdmin,
			))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			uo := f.KubernetesGlobalResource("UserOperation", "user-operation-01")
			Expect(uo.Field("status.phase").String()).To(Equal("Failed"))
			Expect(uo.Field("status.message").String()).NotTo(BeEmpty())

			// Nothing must be deleted: the guard returns before any session is touched.
			for _, name := range []string{"offsess-1", "offsess-2"} {
				offsess := f.KubernetesResource("OfflineSessions", "d8-user-authn", name)
				Expect(offsess.Exists()).To(BeTrue(), "OfflineSessions %s must be untouched", name)
			}
			for _, name := range []string{"rt-1", "rt-2"} {
				rt := f.KubernetesResource("RefreshToken", "d8-user-authn", name)
				Expect(rt.Exists()).To(BeTrue(), "RefreshToken %s must be untouched", name)
			}
		})

	})
})

// Anchored time so success cases can assert exact instants rather than
// "now ± epsilon"; this is the whole reason resolveLockUntil takes `now`
// as a parameter.
var fixedNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestResolveLockUntil(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "permanent sentinel resolves to the year-9999 lock-forever marker",
			input: userOperationLockForever,
			want:  userOperationForeverTime,
		},
		{
			name:  "plain Go duration is added to now",
			input: "30m",
			want:  fixedNow.Add(30 * time.Minute),
		},
		{
			name:  "compound Go duration with no days unit",
			input: "2h30m",
			want:  fixedNow.Add(2*time.Hour + 30*time.Minute),
		},
		{
			name:  "single days segment expands to 24h-per-day",
			input: "7d",
			want:  fixedNow.Add(7 * 24 * time.Hour),
		},
		{
			name:  "fractional days are honoured",
			input: "0.5d",
			want:  fixedNow.Add(12 * time.Hour),
		},
		{
			name:  "days mix freely with other Go-duration units",
			input: "1d12h",
			want:  fixedNow.Add(36 * time.Hour),
		},
		{
			name:    "non-parseable garbage surfaces an error (CRD pattern should make this unreachable in prod)",
			input:   "never",
			wantErr: true,
		},
		{
			name:    "explicitly zero duration is rejected as non-positive (CEL should also catch it)",
			input:   "0s",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveLockUntil(tt.input, fixedNow)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveLockUntil(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !got.Equal(tt.want) {
				t.Errorf("resolveLockUntil(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestApplyUserOperationFilterNeverErrors guards the hard requirement that the
// UserOperation FilterFunc never returns an error: addon-operator runs it while
// loading existing objects to enable the hook's kubernetes bindings, so a single
// malformed object that made the filter error would lock the whole hook queue.
// A valid object must still convert in full; a structurally broken one must
// capture the conversion error in FilterError so getUserOperations can mark it
// Failed instead of the load failing.
func TestApplyUserOperationFilterNeverErrors(t *testing.T) {
	t.Run("valid object converts in full and carries no filter error", func(t *testing.T) {
		res, err := applyUserOperationFilter(&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "deckhouse.io/v1",
			"kind":       "UserOperation",
			"metadata":   map[string]any{"name": "ok"},
			"spec": map[string]any{
				"type":          "Lock",
				"initiatorType": "Admin",
				"user":          "admin",
				"lock":          map[string]any{"for": "1h"},
			},
		}})
		if err != nil {
			t.Fatalf("filter must never return an error, got: %v", err)
		}
		op, ok := res.(*UserOperation)
		if !ok || op == nil {
			t.Fatalf("filter must return a *UserOperation, got %T", res)
		}
		if op.FilterError != "" {
			t.Errorf("valid object must not carry a filter error, got %q", op.FilterError)
		}
		if op.Spec.Type != UserOperationTypeLock || op.Spec.Lock == nil || op.Spec.Lock.For != "1h" {
			t.Errorf("valid object must convert in full, got %+v", op.Spec)
		}
	})

	// Each of these would make sdk.FromUnstructured fail; none may lock the queue,
	// and each must capture the error so the object is later marked Failed.
	brokenObjects := map[string]map[string]any{
		"spec is not an object": {
			"apiVersion": "deckhouse.io/v1",
			"kind":       "UserOperation",
			"metadata":   map[string]any{"name": "broken-spec"},
			"spec":       "this should be an object",
		},
		"spec.type is the wrong type": {
			"apiVersion": "deckhouse.io/v1",
			"kind":       "UserOperation",
			"metadata":   map[string]any{"name": "broken-type"},
			"spec":       map[string]any{"type": 12345},
		},
	}

	for name, obj := range brokenObjects {
		t.Run(name, func(t *testing.T) {
			res, err := applyUserOperationFilter(&unstructured.Unstructured{Object: obj})
			if err != nil {
				t.Fatalf("filter must never return an error (it locks the hook queue), got: %v", err)
			}
			op, ok := res.(*UserOperation)
			if !ok || op == nil {
				t.Fatalf("filter must return a *UserOperation, got %T", res)
			}
			if op.Name == "" {
				t.Errorf("snapshot must preserve metadata.name for the status patch and cleanup")
			}
			if op.FilterError == "" {
				t.Errorf("snapshot must capture the conversion error so the object is marked Failed")
			}
		})
	}
}

// TestExecuteUserOperationFailsOnFilterError verifies that a captured filter
// error is surfaced by executeUserOperation, which is what drives the operation
// into the Failed phase with a precise status.message.
func TestExecuteUserOperationFailsOnFilterError(t *testing.T) {
	op := UserOperation{FilterError: `time: unknown unit "d" in duration "456789d"`}
	err := executeUserOperation(nil, op)
	if err == nil {
		t.Fatal("executeUserOperation must return an error when FilterError is set")
	}
	if !strings.Contains(err.Error(), "456789d") {
		t.Errorf("returned error must carry the original conversion reason, got: %v", err)
	}
}

