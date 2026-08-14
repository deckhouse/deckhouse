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

func projectLocal(pw passwordView, user *userView, now time.Time) desiredAccount {
	locked := timeLockActive(pw.LockedUntil, now)
	lockedByAdmin := locked && hasAdminLock(pw.Annotations)

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
			LockedUntil:            metaTimePtr(pw.LockedUntil),
			LockedByAdministrator:  lockedByAdmin,
			UserRef:                userRef,
			ExpireAt:               expireAt,
			Groups:                 groups,
		},
	}
}

func projectExternal(sess sessionView, providerType string, now time.Time) desiredAccount {
	locked := timeLockActive(sess.LockedUntil, now)
	lockedByAdmin := locked && hasAdminLock(sess.Annotations)

	return desiredAccount{
		Name: naming.ExternalName(sess.ConnID, sess.UserID),
		Labels: map[string]string{
			v1alpha1.LabelKind:        v1alpha1.KindExternal,
			v1alpha1.LabelConnectorID: sess.ConnID,
			v1alpha1.LabelLocked:      strconv.FormatBool(locked),
		},
		Status: v1alpha1.UserAccountStatus{
			Email:                  sess.Email,
			UserID:                 sess.UserID,
			Kind:                   v1alpha1.KindExternal,
			ConnectorID:            sess.ConnID,
			ProviderType:           providerType,
			IncorrectLoginAttempts: sess.Attempts,
			Locked:                 locked,
			LockedUntil:            metaTimePtr(sess.LockedUntil),
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

func isExternalCandidate(sess sessionView) bool {
	if sess.Email == "" || sess.ConnID == "" {
		return false
	}
	return !strings.EqualFold(sess.ConnID, naming.LocalConnectorID)
}
