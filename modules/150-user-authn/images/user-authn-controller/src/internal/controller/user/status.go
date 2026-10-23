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

package user

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/useridentity"
)

type userStatusPatch struct {
	ExpireAt string   `json:"expireAt,omitempty"`
	Groups   []string `json:"groups"`
	Lock     userLock `json:"lock"`
}

// lockSource is a lock in one place and whether an administrator set it.
type lockSource struct {
	until   *time.Time
	byAdmin bool
}

// enforcedLock is the lock Dex enforces on the local user: the one in the UserIdentity of the pair
// (user, local). Until the controller marks that UserIdentity as migrated, the transitional Dex
// patch also reads the lock of the Password, and the later of the two is in force.
func enforcedLock(pw passwordView, ident *useridentity.Identity) (lockSource, error) {
	fromIdentity := lockSource{}
	if ident != nil {
		blocked, err := ident.BlockedUntil()
		if err != nil {
			return lockSource{}, err
		}
		if !blocked.IsZero() {
			fromIdentity = lockSource{until: &blocked, byAdmin: ident.LockedByAdministrator()}
		}
		if ident.Migrated() {
			return fromIdentity, nil
		}
	}
	_, byAdmin := pw.Annotations[lockedByAdministratorAnnot]
	return laterLock(lockSource{until: pw.LockedUntil, byAdmin: byAdmin}, fromIdentity), nil
}

// laterLock is the lock that ends later.
func laterLock(a, b lockSource) lockSource {
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
		return lockSource{until: a.until, byAdmin: a.byAdmin || b.byAdmin}
	}
}

// statusLock is User.status.lock for the lock in force.
func statusLock(lock lockSource, now time.Time) userLock {
	if lock.until == nil || !lock.until.After(now) {
		return userLock{}
	}
	status := userLock{
		State:   true,
		Reason:  lockReasonPasswordPolicy,
		Message: lockMessagePasswordPolicy,
		Until:   lock.until.UTC().Format(time.RFC3339),
	}
	if lock.byAdmin {
		status.Reason = lockReasonAdministrator
		status.Message = lockMessageAdministrator
	}
	return status
}

func expireAtForUser(user userView, now time.Time) (string, error) {
	if user.ExpireAt != "" || user.TTL == "" {
		return user.ExpireAt, nil
	}
	parsed, err := time.ParseDuration(user.TTL)
	if err != nil {
		return user.ExpireAt, fmt.Errorf("parse ttl %q: %w", user.TTL, err)
	}
	return now.Add(parsed).UTC().Format(time.RFC3339), nil
}

func statusUnchanged(user userView, desired userStatusPatch) bool {
	if !user.HasStatus {
		return false
	}
	if !equalStringSets(user.Groups, desired.Groups) {
		return false
	}
	if user.ExpireAt != desired.ExpireAt {
		return false
	}
	return lockEqual(user.Lock, desired.Lock)
}

func lockEqual(a, b userLock) bool {
	return a.State == b.State && a.Reason == b.Reason && a.Message == b.Message && a.Until == b.Until
}

func (r *Reconciler) patchUserStatus(ctx context.Context, user userView, desired userStatusPatch) error {
	if statusUnchanged(user, desired) {
		return nil
	}
	groups := desired.Groups
	if groups == nil {
		groups = []string{}
	}
	status := map[string]any{
		"groups": groups,
		"lock":   lockStatusMap(desired.Lock),
	}
	if desired.ExpireAt != "" {
		status["expireAt"] = desired.ExpireAt
	}
	raw, err := json.Marshal(map[string]any{"status": status})
	if err != nil {
		return fmt.Errorf("marshal user status patch: %w", err)
	}
	obj := controller.Object(controller.UserGVK)
	obj.SetName(user.Name)
	if err := r.client.Status().Patch(ctx, obj, client.RawPatch(types.MergePatchType, raw)); err != nil {
		return fmt.Errorf("patch user %s status: %w", user.Name, err)
	}
	r.log.Info("synced user status", "user", user.Name)
	return nil
}

// lockStatusMap sends explicit nulls for inactive lock fields so a merge-patch
// clears stale reason/message/until. omitempty on userLock would leave them.
func lockStatusMap(lock userLock) map[string]any {
	if !lock.State {
		return map[string]any{
			"state":   false,
			"reason":  nil,
			"message": nil,
			"until":   nil,
		}
	}
	return map[string]any{
		"state":   true,
		"reason":  lock.Reason,
		"message": lock.Message,
		"until":   lock.Until,
	}
}
