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
	"log/slog"
	"os"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestPasswordCache(t *testing.T) {
	scheme := runtime.NewScheme()

	passwordGVR := schema.GroupVersionResource{
		Group:    "dex.coreos.com",
		Version:  "v1",
		Resource: "passwords",
	}

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
	logger := testLogger()

	cache := NewPasswordCache(dynamicClient, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := cache.Start(ctx); err != nil {
		t.Fatalf("Failed to start password cache: %v", err)
	}
	defer cache.Stop()

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
			got := cache.IsLocalUser(tt.username)
			if got != tt.want {
				t.Errorf("IsLocalUser(%q) = %v, want %v", tt.username, got, tt.want)
			}
		})
	}

	if !cache.IsSynced() {
		t.Error("Cache should be synced")
	}

	if cache.Count() != 1 {
		t.Errorf("Cache count = %d, want 1", cache.Count())
	}
}

func TestK8sClient_CreatePasswordResetOperation(t *testing.T) {
	scheme := runtime.NewScheme()

	gvrToListKind := map[schema.GroupVersionResource]string{
		userOperationGVR: "UserOperationList",
	}

	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)
	logger := testLogger()
	client := NewClientWithDynamic(dynamicClient, logger)

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

func TestK8sClient_IsLocalUser_CacheNotSynced(t *testing.T) {
	scheme := runtime.NewScheme()

	passwordGVR := schema.GroupVersionResource{
		Group:    "dex.coreos.com",
		Version:  "v1",
		Resource: "passwords",
	}

	gvrToListKind := map[schema.GroupVersionResource]string{
		passwordGVR: "PasswordList",
	}

	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)
	logger := testLogger()
	client := NewClientWithDynamic(dynamicClient, logger)

	// Don't start the cache, so it's not synced
	_, err := client.IsLocalUser(context.Background(), "admin")
	if err != ErrCacheNotSynced {
		t.Errorf("IsLocalUser() error = %v, want %v", err, ErrCacheNotSynced)
	}
}
