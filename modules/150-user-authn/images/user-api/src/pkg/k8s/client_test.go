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

package k8s

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestK8sClient_IsLocalUser(t *testing.T) {
	scheme := runtime.NewScheme()

	existingPassword := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "dex.coreos.com/v1",
			"kind":       "Password",
			"metadata": map[string]interface{}{
				"name":      "some-encoded-name",
				"namespace": dexNamespace,
			},
			"username": "admin",
			"email":    "admin@example.com",
		},
	}

	gvrToListKind := map[schema.GroupVersionResource]string{
		passwordGVR: "PasswordList",
	}

	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, existingPassword)
	client := NewClientWithDynamic(dynamicClient)

	tests := []struct {
		name     string
		username string
		want     bool
	}{
		{
			name:     "existing local user",
			username: "admin",
			want:     true,
		},
		{
			name:     "non-existing user",
			username: "unknown",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.IsLocalUser(context.Background(), tt.username)
			if err != nil {
				t.Errorf("IsLocalUser() unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("IsLocalUser(%q) = %v, want %v", tt.username, got, tt.want)
			}
		})
	}
}

func TestK8sClient_CreatePasswordResetOperation(t *testing.T) {
	scheme := runtime.NewScheme()

	gvrToListKind := map[schema.GroupVersionResource]string{
		userOperationGVR: "UserOperationList",
	}

	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)
	client := NewClientWithDynamic(dynamicClient)

	_, err := client.CreatePasswordResetOperation(
		context.Background(),
		"testuser",
		"$2y$10$testHash",
	)

	if err != nil {
		t.Fatalf("CreatePasswordResetOperation() unexpected error: %v", err)
	}

	list, err := dynamicClient.Resource(userOperationGVR).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list UserOperations: %v", err)
	}

	if len(list.Items) != 1 {
		t.Errorf("Expected 1 UserOperation, got %d", len(list.Items))
	}

	op := list.Items[0]
	spec, ok := op.Object["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("Failed to get spec from UserOperation")
	}

	if spec["user"] != "testuser" {
		t.Errorf("UserOperation user = %v, want testuser", spec["user"])
	}

	if spec["type"] != "ResetPassword" {
		t.Errorf("UserOperation type = %v, want ResetPassword", spec["type"])
	}

	if spec["initiatorType"] != "self" {
		t.Errorf("UserOperation initiatorType = %v, want self", spec["initiatorType"])
	}

	resetPassword, ok := spec["resetPassword"].(map[string]interface{})
	if !ok {
		t.Fatal("Failed to get resetPassword from spec")
	}

	if resetPassword["newPasswordHash"] != "$2y$10$testHash" {
		t.Errorf("UserOperation newPasswordHash = %v, want $2y$10$testHash", resetPassword["newPasswordHash"])
	}
}
