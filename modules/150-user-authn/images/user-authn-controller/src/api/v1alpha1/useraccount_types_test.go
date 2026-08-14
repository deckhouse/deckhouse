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

package v1alpha1

import (
	"encoding/json"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUserAccountJSONOmitsSecrets(t *testing.T) {
	t.Parallel()

	now := metav1.NewTime(time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC))

	tests := []struct {
		name    string
		account UserAccount
	}{
		{
			name: "filled local account",
			account: UserAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: SchemeGroupVersion.String(),
					Kind:       "UserAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "jane.doe",
					Labels: map[string]string{
						LabelKind:   KindLocal,
						LabelLocked: "true",
					},
				},
				Status: UserAccountStatus{
					Email:                  "jane.doe@example.com",
					Username:               "jane.doe",
					UserID:                 "user-1",
					Kind:                   KindLocal,
					IncorrectLoginAttempts: 3,
					Locked:                 true,
					LockedUntil:            now.DeepCopy(),
					LockedByAdministrator:  true,
					UserRef:                "jane.doe",
					ExpireAt:               now.DeepCopy(),
					Groups:                 []string{"admins"},
				},
			},
		},
		{
			name: "filled external account",
			account: UserAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: SchemeGroupVersion.String(),
					Kind:       "UserAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "ldap-alice",
					Labels: map[string]string{
						LabelKind:        KindExternal,
						LabelConnectorID: "ldap-main",
						LabelLocked:      "false",
					},
				},
				Status: UserAccountStatus{
					Email:                  "alice@example.com",
					Username:               "alice",
					UserID:                 "user-2",
					Kind:                   KindExternal,
					ConnectorID:            "ldap-main",
					ProviderType:           "LDAP",
					IncorrectLoginAttempts: 1,
					Groups:                 []string{"developers"},
				},
			},
		},
	}

	forbiddenKeys := []string{"hash", "previousHashes", "connectorData", "refresh", "totp", "password"}
	requiredStatusKeys := []string{"email", "incorrectLoginAttempts", "kind"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			raw, err := json.Marshal(tt.account)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}

			var decoded any
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatalf("json.Unmarshal: %v", err)
			}

			keys := collectJSONKeys(decoded)
			for _, key := range forbiddenKeys {
				if _, ok := keys[key]; ok {
					t.Errorf("marshaled JSON contains forbidden key %q: %s", key, raw)
				}
			}

			obj, ok := decoded.(map[string]any)
			if !ok {
				t.Fatalf("top-level JSON is %T, want object", decoded)
			}
			status, ok := obj["status"].(map[string]any)
			if !ok {
				t.Fatalf("status is %T, want object", obj["status"])
			}
			for _, key := range requiredStatusKeys {
				if _, ok := status[key]; !ok {
					t.Errorf("status missing required key %q: %s", key, raw)
				}
			}
		})
	}
}

func collectJSONKeys(v any) map[string]struct{} {
	keys := make(map[string]struct{})
	collectJSONKeysInto(v, keys)
	return keys
}

func collectJSONKeysInto(v any, keys map[string]struct{}) {
	switch typed := v.(type) {
	case map[string]any:
		for key, child := range typed {
			keys[key] = struct{}{}
			collectJSONKeysInto(child, keys)
		}
	case []any:
		for _, child := range typed {
			collectJSONKeysInto(child, keys)
		}
	}
}
