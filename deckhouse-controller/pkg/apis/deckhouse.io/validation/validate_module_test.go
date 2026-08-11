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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
)

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

// TestModuleV1alpha2ValidationHandler verifies that a module whose ModuleConfig exists
// can be neither changed nor deleted by hand, that both are allowed without a config, and
// that the deckhouse service account is exempt.
func TestModuleV1alpha2ValidationHandler(t *testing.T) {
	const moduleName = "test-module"

	module := &v1alpha2.Module{ObjectMeta: metav1.ObjectMeta{Name: moduleName}}
	moduleConfig := &v1alpha1.ModuleConfig{ObjectMeta: metav1.ObjectMeta{Name: moduleName}}

	tests := []struct {
		name        string
		operation   string
		username    string
		withConfig  bool
		wantAllowed bool
		wantMessage string
	}{
		{
			name:        "editing a module managed by a module config is forbidden",
			operation:   "UPDATE",
			username:    "kubernetes-admin",
			withConfig:  true,
			wantAllowed: false,
			wantMessage: "manual Module change is forbidden",
		},
		{
			name:        "editing a module without a module config is allowed",
			operation:   "UPDATE",
			username:    "kubernetes-admin",
			wantAllowed: true,
		},
		{
			name:        "deleting a module managed by a module config is forbidden",
			operation:   "DELETE",
			username:    "kubernetes-admin",
			withConfig:  true,
			wantAllowed: false,
			wantMessage: "manual Module deletion is forbidden",
		},
		{
			name:        "deleting a module without a module config is allowed",
			operation:   "DELETE",
			username:    "kubernetes-admin",
			wantAllowed: true,
		},
		{
			name:        "deckhouse service account is exempt",
			operation:   "UPDATE",
			username:    deckhouseServiceAccount,
			withConfig:  true,
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []client.Object
			if tt.withConfig {
				objs = append(objs, moduleConfig)
			}

			handler := moduleV1alpha2ValidationHandler(newFakeClient(t, objs...))
			resp := callHandler(t, handler, newModuleV1alpha2AdmissionReview(t, tt.operation, tt.username, module))

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

// TestModuleV1alpha2ValidationHandler_ConfigLookupFails covers the case the webhook's
// failurePolicy: Fail exists for: when the module config cannot be read, the handler
// must surface an error rather than mistake it for an absent module config and allow
// the change.
func TestModuleV1alpha2ValidationHandler_ConfigLookupFails(t *testing.T) {
	module := &v1alpha2.Module{ObjectMeta: metav1.ObjectMeta{Name: "test-module"}}

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	require.NoError(t, v1alpha2.AddToScheme(scheme))

	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return errors.New("connection refused")
			},
		}).
		Build()

	handler := moduleV1alpha2ValidationHandler(cli)
	review := newModuleV1alpha2AdmissionReview(t, "UPDATE", "kubernetes-admin", module)

	body, err := json.Marshal(review)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func newModuleV1alpha2AdmissionReview(t *testing.T, operation, username string, module *v1alpha2.Module) *admissionv1.AdmissionReview {
	t.Helper()

	review := newModuleAdmissionReview(operation, username, nil)

	raw, err := json.Marshal(module)
	require.NoError(t, err)

	// on delete the API server sends the object in oldObject, kubewebhook falls back
	// to it when object is empty
	if operation == "DELETE" {
		review.Request.OldObject = runtime.RawExtension{Raw: raw}

		return review
	}

	review.Request.Object = runtime.RawExtension{Raw: raw}

	return review
}
