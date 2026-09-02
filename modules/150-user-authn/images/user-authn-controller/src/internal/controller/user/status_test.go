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
)

func TestLockFromPassword(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	until := now.Add(time.Hour)

	tests := []struct {
		name string
		pw   passwordView
		want userLock
	}{
		{name: "no lock"},
		{
			name: "expired lock",
			pw:   passwordView{LockedUntil: ptrTime(now.Add(-time.Second))},
		},
		{
			name: "policy lockout",
			pw:   passwordView{LockedUntil: &until},
			want: userLock{
				State:   true,
				Reason:  lockReasonPasswordPolicy,
				Message: lockMessagePasswordPolicy,
				Until:   until.UTC().Format(time.RFC3339),
			},
		},
		{
			name: "admin lock",
			pw: passwordView{
				LockedUntil: &until,
				Annotations: map[string]string{lockedByAdministratorAnnot: ""},
			},
			want: userLock{
				State:   true,
				Reason:  lockReasonAdministrator,
				Message: lockMessageAdministrator,
				Until:   until.UTC().Format(time.RFC3339),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := lockFromPassword(tt.pw, now)
			if !lockEqual(got, tt.want) {
				t.Errorf("lockFromPassword() = %+v, want %+v", got, tt.want)
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
