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
)

type userStatusPatch struct {
	ExpireAt string   `json:"expireAt,omitempty"`
	Groups   []string `json:"groups"`
	Lock     userLock `json:"lock"`
}

func lockFromPassword(pw passwordView, now time.Time) userLock {
	if pw.LockedUntil != nil && pw.LockedUntil.After(now) {
		lock := userLock{
			State:   true,
			Reason:  lockReasonPasswordPolicy,
			Message: lockMessagePasswordPolicy,
			Until:   pw.LockedUntil.UTC().Format(time.RFC3339),
		}
		if _, ok := pw.Annotations[lockedByAdministratorAnnot]; ok {
			lock.Reason = lockReasonAdministrator
			lock.Message = lockMessageAdministrator
		}
		return lock
	}
	return userLock{}
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
	if desired.Groups == nil {
		desired.Groups = []string{}
	}
	raw, err := json.Marshal(map[string]any{"status": desired})
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
