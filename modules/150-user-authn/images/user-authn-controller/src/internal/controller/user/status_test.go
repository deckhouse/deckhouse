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
	"testing"
	"time"

	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/useridentity"
)

func TestEnforcedLock(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	until := now.Add(time.Hour)
	later := now.Add(2 * time.Hour)
	policyLock := userLock{
		State:   true,
		Reason:  lockReasonPasswordPolicy,
		Message: lockMessagePasswordPolicy,
		Until:   until.UTC().Format(time.RFC3339),
	}
	adminLock := func(t time.Time) userLock {
		return userLock{
			State:   true,
			Reason:  lockReasonAdministrator,
			Message: lockMessageAdministrator,
			Until:   t.UTC().Format(time.RFC3339),
		}
	}
	identity := func(blocked time.Time, admin, migrated bool) *useridentity.Identity {
		id := useridentity.New("jane", naming.LocalConnectorID)
		if !blocked.IsZero() {
			if err := id.SetBlockedUntil(blocked); err != nil {
				t.Fatal(err)
			}
		}
		if admin {
			id.SetAdminLock(blocked)
		}
		if migrated {
			if err := id.SetRecord(useridentity.Record{Time: now.Format(time.RFC3339)}); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}

	tests := []struct {
		name  string
		pw    passwordView
		ident *useridentity.Identity
		want  userLock
	}{
		{name: "no lock"},
		{
			name: "expired Password lock",
			pw:   passwordView{LockedUntil: ptrTime(now.Add(-time.Second))},
		},
		{
			name: "Password lock before the copy",
			pw:   passwordView{LockedUntil: &until},
			want: policyLock,
		},
		{
			name: "Password admin lock before the copy",
			pw: passwordView{
				LockedUntil: &until,
				Annotations: map[string]string{lockedByAdministratorAnnot: ""},
			},
			want: adminLock(until),
		},
		{
			name:  "later UserIdentity lock before the copy",
			pw:    passwordView{LockedUntil: &until},
			ident: identity(later, true, false),
			want:  adminLock(later),
		},
		{
			name:  "migrated UserIdentity without a lock: the Password lock is history",
			pw:    passwordView{LockedUntil: &until},
			ident: identity(time.Time{}, false, true),
		},
		{
			name:  "migrated UserIdentity lock",
			ident: identity(until, false, true),
			want:  policyLock,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lock, err := enforcedLock(tt.pw, tt.ident)
			if err != nil {
				t.Fatalf("enforcedLock: %v", err)
			}
			if got := statusLock(lock, now); !lockEqual(got, tt.want) {
				t.Errorf("status lock = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestLockStatusMapClearsInactiveFields(t *testing.T) {
	t.Parallel()

	inactive := lockStatusMap(userLock{})
	for _, key := range []string{"reason", "message", "until"} {
		if inactive[key] != nil {
			t.Errorf("inactive %s = %v, want nil so merge-patch clears the field", key, inactive[key])
		}
	}
	if inactive["state"] != false {
		t.Errorf("inactive state = %v, want false", inactive["state"])
	}

	active := lockStatusMap(userLock{
		State:   true,
		Reason:  lockReasonAdministrator,
		Message: lockMessageAdministrator,
		Until:   "2026-08-19T13:00:00Z",
	})
	if active["state"] != true || active["reason"] != lockReasonAdministrator {
		t.Errorf("active lock map = %#v", active)
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
