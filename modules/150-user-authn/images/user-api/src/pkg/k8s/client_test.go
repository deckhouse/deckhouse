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
	"errors"
	"log/slog"
	"os"
	"reflect"
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

func newFakeDynamicClient(objects ...runtime.Object) *fake.FakeDynamicClient {
	gvrToListKind := map[schema.GroupVersionResource]string{
		passwordGVR:      "PasswordList",
		userOperationGVR: "UserOperationList",
		userAccountGVR:   "UserAccountList",
	}
	return fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, objects...)
}

func password(name, username, userID string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "dex.coreos.com/v1",
			"kind":       "Password",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": dexNamespace,
			},
			"username": username,
			"email":    username + "@example.com",
			"hash":     "JDJhJDEwJGZ5Q2o3dW5rQnh5eFNyaDZqVUxrVE93dlVhYk56ZGQ4MWFtMDJDVDl0V1RsYi55MEVwdHJp",
		},
	}
	if userID != "" {
		obj.Object["userID"] = userID
	}
	return obj
}

func startedClient(t *testing.T, dynamicClient *fake.FakeDynamicClient) *K8sClient {
	t.Helper()

	client, err := NewClientWithDynamic(dynamicClient, testLogger())
	if err != nil {
		t.Fatalf("NewClientWithDynamic() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(client.Stop)

	return client
}

func TestPasswordCache_Username(t *testing.T) {
	client := startedClient(t, newFakeDynamicClient(
		password("admin-obj", "admin", "admin"),
		password("jane-obj", "jane", "jane-id"),
		password("legacy-obj", "legacy", ""),
		password("dup-1", "dup-one", "dup"),
		password("dup-2", "dup-two", "dup"),
	))

	tests := []struct {
		name    string
		userID  string
		want    string
		wantErr error
	}{
		{name: "user ID equal to the username", userID: "admin", want: "admin"},
		{name: "user ID that differs from the username", userID: "jane-id", want: "jane"},
		{name: "username of a user with another user ID", userID: "jane", wantErr: ErrUserNotFound},
		{name: "object without a user ID is found by username", userID: "legacy", want: "legacy"},
		{name: "two objects with one user ID", userID: "dup", wantErr: ErrAmbiguousUser},
		{name: "unknown user ID", userID: "unknown", wantErr: ErrUserNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.LocalUsername(context.Background(), tt.userID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("LocalUsername(%q) error = %v, want %v", tt.userID, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("LocalUsername(%q) = %q, want %q", tt.userID, got, tt.want)
			}
		})
	}

	if got := client.passwordCache.Count(); got != 5 {
		t.Errorf("Count() = %d, want 5", got)
	}
}

func TestPasswordCache_KeepsNoHashes(t *testing.T) {
	client := startedClient(t, newFakeDynamicClient(password("jane-obj", "jane", "jane")))

	for _, obj := range client.passwordCache.informer.GetStore().List() {
		// cachedPassword has no field for the hash or the email.
		if _, ok := obj.(*cachedPassword); !ok {
			t.Fatalf("cached object is %T, want *cachedPassword", obj)
		}
	}
}

func TestPasswordCache_FollowsUpdates(t *testing.T) {
	dynamicClient := newFakeDynamicClient(password("jane-obj", "jane", "jane"))
	client := startedClient(t, dynamicClient)

	ctx := context.Background()
	if _, err := dynamicClient.Resource(passwordGVR).Namespace(dexNamespace).
		Update(ctx, password("jane-obj", "jane", "jane-renamed"), metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update Password: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		username, err := client.LocalUsername(ctx, "jane-renamed")
		if err == nil && username == "jane" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("LocalUsername(jane-renamed) = %q, %v after the update, want jane", username, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, err := client.LocalUsername(ctx, "jane"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("LocalUsername(jane) error = %v after the update, want %v", err, ErrUserNotFound)
	}
}

func TestPasswordCache_StopIsIdempotent(t *testing.T) {
	client := startedClient(t, newFakeDynamicClient())

	// Repeated Stop must not panic on closing an already-closed channel.
	client.Stop()
	client.Stop()
}

func TestK8sClient_AccountName(t *testing.T) {
	client := startedClient(t, newFakeDynamicClient(password("jane-obj", "jane", "jane-id")))

	local, err := client.AccountName(context.Background(), "jane-id", "local")
	if err != nil {
		t.Fatalf("AccountName() of a local user error = %v", err)
	}
	if want := LocalAccountName("jane@example.com", "jane"); local != want {
		t.Errorf("AccountName() of a local user = %q, want %q", local, want)
	}
	if _, err := client.AccountName(context.Background(), "nobody", "local"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("AccountName() of an unknown local user error = %v, want %v", err, ErrUserNotFound)
	}

	external, err := client.AccountName(context.Background(), "cn=jdoe,ou=people", "ldap")
	if err != nil {
		t.Fatalf("AccountName() of an external user error = %v", err)
	}
	if want := ExternalAccountName("ldap", "cn=jdoe,ou=people"); external != want {
		t.Errorf("AccountName() of an external user = %q, want %q", external, want)
	}
}

func userAccount(name, userAgent string) *unstructured.Unstructured {
	return pairAccount(name, "jane", userAgent)
}

func pairAccount(name, userID, userAgent string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "UserAccount",
		"metadata":   map[string]interface{}{"name": name},
		"status": map[string]interface{}{
			"email":       "jane@example.com",
			"groups":      []interface{}{"admins"},
			"userID":      userID,
			"connectorID": "local",
			"sessions": []interface{}{map[string]interface{}{
				"id":         "fehzwdh5qy4oyqcn7qhkcgzbm",
				"kind":       "Browser",
				"clients":    []interface{}{"console"},
				"ipAddress":  "203.0.113.7",
				"userAgent":  userAgent,
				"signedInAt": "2026-10-16T10:00:00Z",
				"endsAt":     "2026-10-17T10:00:00Z",
			}},
			"sessionsTruncated": true,
		},
	}}
}

// The devices come from the cache, found by the pair of the account: once it has synced, a request
// makes no call to the API server.
func TestK8sClient_Devices(t *testing.T) {
	dynamicClient := newFakeDynamicClient(userAccount("local-jane", "Firefox"), pairAccount("local-john", "john", "Safari"))
	client := startedClient(t, dynamicClient)
	if !client.Synced() {
		t.Fatal("Synced() = false after Start")
	}
	dynamicClient.ClearActions()

	got, err := client.Devices(context.Background(), "jane", "local")
	if err != nil {
		t.Fatalf("Devices() error = %v", err)
	}
	signedInAt := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)
	endsAt := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	want := Devices{
		Items: []Device{{
			ID: "fehzwdh5qy4oyqcn7qhkcgzbm", Kind: DeviceKindBrowser, Clients: []string{"console"},
			IPAddress: "203.0.113.7", UserAgent: "Firefox", SignedInAt: signedInAt, EndsAt: endsAt,
		}},
		Truncated:   true,
		UserID:      "jane",
		ConnectorID: "local",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Devices() = %+v, want %+v", got, want)
	}

	for _, pair := range [][2]string{{"nobody", "local"}, {"jane", "ldap"}} {
		none, err := client.Devices(context.Background(), pair[0], pair[1])
		if err != nil || len(none.Items) != 0 {
			t.Errorf("Devices(%v) of a missing account = %+v, %v; want none", pair, none, err)
		}
	}
	if actions := dynamicClient.Actions(); len(actions) != 0 {
		t.Errorf("Devices() called the API server: %v", actions)
	}
}

func TestAccountCache_KeepsOnlyTheDevices(t *testing.T) {
	client := startedClient(t, newFakeDynamicClient(userAccount("local-jane", "Firefox")))

	for _, obj := range client.accountCache.informer.GetStore().List() {
		account, ok := obj.(*cachedAccount)
		if !ok {
			t.Fatalf("cached object is %T, want *cachedAccount", obj)
		}
		if again, _ := client.accountCache.trimAccount(account); again != account {
			t.Error("trimming a trimmed account changed it")
		}
	}
}

func TestAccountCache_FollowsUpdates(t *testing.T) {
	dynamicClient := newFakeDynamicClient(userAccount("local-jane", "Firefox"))
	client := startedClient(t, dynamicClient)

	ctx := context.Background()
	if _, err := dynamicClient.Resource(userAccountGVR).Update(ctx, userAccount("local-jane", "Safari"), metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update UserAccount: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		got, err := client.Devices(ctx, "jane", "local")
		if err == nil && len(got.Items) == 1 && got.Items[0].UserAgent == "Safari" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Devices() = %+v, %v after the update, want the Safari device", got, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestK8sClient_Devices_CacheNotSynced(t *testing.T) {
	client, err := NewClientWithDynamic(newFakeDynamicClient(userAccount("local-jane", "Firefox")), testLogger())
	if err != nil {
		t.Fatalf("NewClientWithDynamic() error = %v", err)
	}
	if _, err := client.Devices(context.Background(), "jane", "local"); !errors.Is(err, ErrCacheNotSynced) {
		t.Errorf("Devices() error = %v, want %v", err, ErrCacheNotSynced)
	}
	if client.Synced() {
		t.Error("Synced() = true before Start")
	}
}

// Two accounts of one pair would mix the devices of two users: neither is listed.
func TestK8sClient_DevicesOfAnAmbiguousPair(t *testing.T) {
	client := startedClient(t, newFakeDynamicClient(userAccount("local-jane", "Firefox"), userAccount("local-jane-2", "Safari")))
	if _, err := client.Devices(context.Background(), "jane", "local"); !errors.Is(err, ErrAmbiguousUser) {
		t.Errorf("Devices() error = %v, want %v", err, ErrAmbiguousUser)
	}
}

func TestK8sClient_LocalUsername_CacheNotSynced(t *testing.T) {
	client, err := NewClientWithDynamic(newFakeDynamicClient(password("admin-obj", "admin", "admin")), testLogger())
	if err != nil {
		t.Fatalf("NewClientWithDynamic() error = %v", err)
	}

	// Don't start the cache, so it's not synced
	if _, err := client.LocalUsername(context.Background(), "admin"); !errors.Is(err, ErrCacheNotSynced) {
		t.Errorf("LocalUsername() error = %v, want %v", err, ErrCacheNotSynced)
	}
}

func TestK8sClient_CreateUserOperation(t *testing.T) {
	tests := []struct {
		name     string
		op       UserOperation
		wantSpec map[string]interface{}
	}{
		{
			name: "password change of a local user",
			op: UserOperation{
				GenerateName: "self-password-change-",
				Type:         OperationTypeRequirePasswordChange,
				User:         "jane",
			},
			wantSpec: map[string]interface{}{
				"type":          "RequirePasswordChange",
				"user":          "jane",
				"initiatorType": "self",
			},
		},
		{
			name: "logout of a local user",
			op: UserOperation{
				GenerateName: "self-logout-",
				Type:         OperationTypeLogout,
				User:         "jane",
			},
			wantSpec: map[string]interface{}{
				"type":          "Logout",
				"user":          "jane",
				"initiatorType": "self",
			},
		},
		{
			name: "logout of an external account",
			op: UserOperation{
				GenerateName: "self-logout-",
				Type:         OperationTypeLogout,
				Target: &Target{
					Email:       "jdoe@example.org",
					ConnectorID: "ldap",
					UserID:      "cn=jdoe,ou=people,dc=example,dc=org",
				},
			},
			wantSpec: map[string]interface{}{
				"type":          "Logout",
				"initiatorType": "self",
				"target": map[string]interface{}{
					"email":       "jdoe@example.org",
					"connectorID": "ldap",
					"userID":      "cn=jdoe,ou=people,dc=example,dc=org",
				},
			},
		},
		{
			name: "logout of one device",
			op: UserOperation{
				GenerateName: "self-logout-device-",
				Type:         OperationTypeLogout,
				User:         "jane",
				Logout:       &Logout{Grant: "qx3w2ttiyq5ovzvbuehdcbbse"},
			},
			wantSpec: map[string]interface{}{
				"type":          "Logout",
				"user":          "jane",
				"initiatorType": "self",
				"logout":        map[string]interface{}{"grant": "qx3w2ttiyq5ovzvbuehdcbbse"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dynamicClient := newFakeDynamicClient()
			client, err := NewClientWithDynamic(dynamicClient, testLogger())
			if err != nil {
				t.Fatalf("NewClientWithDynamic() error = %v", err)
			}

			if _, err := client.CreateUserOperation(context.Background(), tt.op); err != nil {
				t.Fatalf("CreateUserOperation() error = %v", err)
			}

			list, err := dynamicClient.Resource(userOperationGVR).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				t.Fatalf("list UserOperations: %v", err)
			}
			if len(list.Items) != 1 {
				t.Fatalf("got %d UserOperations, want 1", len(list.Items))
			}

			op := list.Items[0]
			if op.GetGenerateName() != tt.op.GenerateName {
				t.Errorf("generateName = %q, want %q", op.GetGenerateName(), tt.op.GenerateName)
			}
			if op.GetAPIVersion() != "deckhouse.io/v1" || op.GetKind() != "UserOperation" {
				t.Errorf("object is %s %s, want deckhouse.io/v1 UserOperation", op.GetAPIVersion(), op.GetKind())
			}
			if !reflect.DeepEqual(op.Object["spec"], tt.wantSpec) {
				t.Errorf("spec = %v, want %v", op.Object["spec"], tt.wantSpec)
			}
		})
	}
}

func TestK8sClient_UserOperationStatus(t *testing.T) {
	operation := func(name string, status map[string]interface{}) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "deckhouse.io/v1",
			"kind":       "UserOperation",
			"metadata":   map[string]interface{}{"name": name},
			"spec":       map[string]interface{}{"type": "Logout", "user": "jane", "initiatorType": "self"},
		}}
		if status != nil {
			obj.Object["status"] = status
		}
		return obj
	}

	client, err := NewClientWithDynamic(newFakeDynamicClient(
		operation("running", nil),
		operation("done", map[string]interface{}{"phase": "Succeeded", "message": "generation 4"}),
		operation("refused", map[string]interface{}{"phase": "Failed", "message": "cannot find password for user: jane"}),
	), testLogger())
	if err != nil {
		t.Fatalf("NewClientWithDynamic() error = %v", err)
	}

	tests := []struct {
		name    string
		want    OperationStatus
		wantErr bool
	}{
		{name: "running", want: OperationStatus{}},
		{name: "done", want: OperationStatus{Phase: PhaseSucceeded, Message: "generation 4"}},
		{name: "refused", want: OperationStatus{Phase: PhaseFailed, Message: "cannot find password for user: jane"}},
		{name: "absent", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.UserOperationStatus(context.Background(), tt.name)
			if (err != nil) != tt.wantErr {
				t.Fatalf("UserOperationStatus(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("UserOperationStatus(%q) = %+v, want %+v", tt.name, got, tt.want)
			}
		})
	}
}

// BenchmarkK8sClient_Devices reads the devices of an account from the cache, as every request of
// user-api does.
func BenchmarkK8sClient_Devices(b *testing.B) {
	client, err := NewClientWithDynamic(newFakeDynamicClient(userAccount("local-jane", "Firefox")), testLogger())
	if err != nil {
		b.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Start(ctx); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(client.Stop)

	for b.Loop() {
		if _, err := client.Devices(context.Background(), "jane", "local"); err != nil {
			b.Fatal(err)
		}
	}
}
