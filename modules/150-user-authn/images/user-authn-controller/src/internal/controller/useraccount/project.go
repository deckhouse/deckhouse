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

package useraccount

import (
	"slices"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"user-authn-controller/api/v1alpha1"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

type desiredAccount struct {
	Name   string
	Labels map[string]string
	Owner  *metav1.OwnerReference
	Status v1alpha1.UserAccountStatus
}

// lockState is a lock in one place and whether an administrator set it.
type lockState struct {
	until   *time.Time
	byAdmin bool
}

// laterLock is the lock that ends later. During 1.78 a lock lives in two places, the old one
// (Password or OfflineSessions) and UserIdentity, and the later one is the one in force.
func laterLock(a, b lockState) lockState {
	switch {
	case a.until == nil:
		return b
	case b.until == nil:
		return a
	case a.until.After(*b.until):
		return a
	case b.until.After(*a.until):
		return b
	default:
		return lockState{until: a.until, byAdmin: a.byAdmin || b.byAdmin}
	}
}

func identityLock(ident *identityView) lockState {
	if ident == nil {
		return lockState{}
	}
	return lockState{until: ident.BlockedUntil, byAdmin: ident.LockedByAdmin}
}

// externalSource is what is known of an external account: its UserIdentity, its OfflineSessions,
// or both.
type externalSource struct {
	UserID   string
	ConnID   string
	Email    string
	Attempts int64
	Lock     lockState
}

func newExternalSource(sess *sessionView, ident *identityView) externalSource {
	var src externalSource
	if sess != nil {
		src = externalSource{
			UserID:   sess.UserID,
			ConnID:   sess.ConnID,
			Email:    sess.Email,
			Attempts: sess.Attempts,
			Lock:     lockState{until: sess.LockedUntil, byAdmin: hasAdminLock(sess.Annotations)},
		}
	}
	if ident == nil {
		return src
	}
	src.UserID, src.ConnID = ident.UserID, ident.ConnectorID
	if src.Email == "" {
		src.Email = ident.Email
	}
	src.Lock = laterLock(src.Lock, identityLock(ident))
	return src
}

func projectLocal(pw passwordView, ident *identityView, user *userView, now time.Time) desiredAccount {
	lock := laterLock(lockState{until: pw.LockedUntil, byAdmin: hasAdminLock(pw.Annotations)}, identityLock(ident))
	locked := timeLockActive(lock.until, now)
	lockedByAdmin := locked && lock.byAdmin

	groups := slices.Clone(pw.Groups)
	userRef := ""
	var expireAt *metav1.Time
	var owner *metav1.OwnerReference
	if user != nil {
		userRef = user.Name
		if user.Groups != nil {
			groups = slices.Clone(user.Groups)
		}
		expireAt = metaTimePtr(user.ExpireAt)
		owner = &metav1.OwnerReference{
			APIVersion:         controller.UserGVK.GroupVersion().String(),
			Kind:               controller.UserGVK.Kind,
			Name:               user.Name,
			UID:                user.UID,
			Controller:         boolPtr(true),
			BlockOwnerDeletion: boolPtr(false),
		}
	}

	return desiredAccount{
		Name: naming.LocalName(localNameInput(pw.Email, pw.Username)),
		Labels: map[string]string{
			v1alpha1.LabelKind:        v1alpha1.KindLocal,
			v1alpha1.LabelConnectorID: naming.LocalConnectorID,
			v1alpha1.LabelLocked:      strconv.FormatBool(locked),
		},
		Owner: owner,
		Status: v1alpha1.UserAccountStatus{
			Email:                  pw.Email,
			Username:               pw.Username,
			UserID:                 pw.UserID,
			Kind:                   v1alpha1.KindLocal,
			ConnectorID:            naming.LocalConnectorID,
			IncorrectLoginAttempts: pw.Attempts,
			Locked:                 locked,
			LockedUntil:            metaTimePtr(lock.until),
			LockedByAdministrator:  lockedByAdmin,
			UserRef:                userRef,
			ExpireAt:               expireAt,
			Groups:                 groups,
		},
	}
}

func projectExternal(src externalSource, providerType string, now time.Time) desiredAccount {
	locked := timeLockActive(src.Lock.until, now)
	lockedByAdmin := locked && src.Lock.byAdmin

	return desiredAccount{
		Name: naming.ExternalName(src.ConnID, src.UserID),
		Labels: map[string]string{
			v1alpha1.LabelKind:        v1alpha1.KindExternal,
			v1alpha1.LabelConnectorID: src.ConnID,
			v1alpha1.LabelLocked:      strconv.FormatBool(locked),
		},
		Status: v1alpha1.UserAccountStatus{
			Email:                  src.Email,
			UserID:                 src.UserID,
			Kind:                   v1alpha1.KindExternal,
			ConnectorID:            src.ConnID,
			ProviderType:           providerType,
			IncorrectLoginAttempts: src.Attempts,
			Locked:                 locked,
			LockedUntil:            metaTimePtr(src.Lock.until),
			LockedByAdministrator:  lockedByAdmin,
		},
	}
}

func localNameInput(email, username string) string {
	if email != "" {
		return email
	}
	return username
}

func timeLockActive(until *time.Time, now time.Time) bool {
	return until != nil && until.After(now)
}

func metaTimePtr(t *time.Time) *metav1.Time {
	if t == nil {
		return nil
	}
	mt := metav1.NewTime(t.UTC())
	return &mt
}

func boolPtr(v bool) *bool {
	return &v
}

func matchUserByEmail(users []userView, email string) *userView {
	if email == "" {
		return nil
	}
	for i := range users {
		if strings.EqualFold(users[i].Email, email) {
			return &users[i]
		}
	}
	return nil
}

func isProjectablePassword(pw passwordView) bool {
	return pw.Email != "" || pw.Username != ""
}

// isExternalConnector tells whether accounts of connID are external: every connector but local.
func isExternalConnector(connID string) bool {
	return connID != "" && !strings.EqualFold(connID, naming.LocalConnectorID)
}
