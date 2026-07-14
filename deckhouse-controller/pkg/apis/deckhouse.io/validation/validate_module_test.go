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

package validation

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

const deckhouseServiceAccount = "system:serviceaccount:d8-system:deckhouse"

func newModuleAdmissionReview(operation, username string, module *v1alpha1.Module) *admissionv1.AdmissionReview {
	review := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Operation: admissionv1.Operation(operation),
			UserInfo:  authenticationv1.UserInfo{Username: username},
		},
	}

	if module != nil {
		raw, _ := json.Marshal(module)
		review.Request.Object = runtime.RawExtension{Raw: raw}
	}

	return review
}

// TestModuleValidationHandler verifies that only the deckhouse service account is
// allowed to mutate Module objects; every other identity is rejected.
func TestModuleValidationHandler(t *testing.T) {
	module := &v1alpha1.Module{ObjectMeta: metav1.ObjectMeta{Name: "test-module"}}

	tests := []struct {
		name        string
		operation   string
		username    string
		wantAllowed bool
		wantMessage string
	}{
		{
			name:        "deckhouse service account is allowed to create",
			operation:   "CREATE",
			username:    deckhouseServiceAccount,
			wantAllowed: true,
		},
		{
			name:        "deckhouse service account is allowed to update",
			operation:   "UPDATE",
			username:    deckhouseServiceAccount,
			wantAllowed: true,
		},
		{
			name:        "regular user is forbidden from changing modules",
			operation:   "UPDATE",
			username:    "system:serviceaccount:default:some-user",
			wantAllowed: false,
			wantMessage: "manual Module change is forbidden",
		},
		{
			name:        "empty username is forbidden",
			operation:   "CREATE",
			username:    "",
			wantAllowed: false,
			wantMessage: "manual Module change is forbidden",
		},
		{
			name:        "cluster admin is forbidden from changing modules",
			operation:   "DELETE",
			username:    "kubernetes-admin",
			wantAllowed: false,
			wantMessage: "manual Module change is forbidden",
		},
	}

	handler := moduleValidationHandler()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := module
			if tt.operation == "DELETE" {
				obj = nil
			}

			review := newModuleAdmissionReview(tt.operation, tt.username, obj)
			if tt.operation == "DELETE" {
				review.Request.OldObject = func() runtime.RawExtension {
					raw, _ := json.Marshal(module)
					return runtime.RawExtension{Raw: raw}
				}()
			}

			resp := callHandler(t, handler, review)

			if tt.wantAllowed {
				assert.True(t, resp.Allowed)
				return
			}

			require.False(t, resp.Allowed)
			require.NotNil(t, resp.Result)
			assert.Contains(t, resp.Result.Message, tt.wantMessage)
		})
	}
}
