//go:build load

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

package api

// The load stand of user-api: U users, each with a UserAccount of D devices and a Password, behind
// the real k8s client on a fake API server. Run it with
//
//	go test -tags load -run '^$' -bench SelfDevices -benchmem ./pkg/api/
//	go test -tags load -run TestLoadMemory -v ./pkg/api/
//
// The benchmark counts the calls the requests make to the API server once the caches have synced.

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"

	"user-api/pkg/auth"
	"user-api/pkg/k8s"
)

// tokenVerifier takes the token for the user ID of a local user.
type tokenVerifier struct{ mockVerifier }

func (*tokenVerifier) Verify(_ context.Context, token string) (*auth.Claims, error) {
	return &auth.Claims{UserID: token, ConnectorID: "local", Email: token + "@example.com", SessionID: "s" + token + "x0"}, nil
}

func loadObjects(users, devices int) []k8sruntime.Object {
	objs := make([]k8sruntime.Object, 0, 2*users)
	signedIn := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC).Format(time.RFC3339)
	for i := range users {
		userID := fmt.Sprintf("user%d", i)
		email := userID + "@example.com"
		sessions := make([]any, 0, devices)
		for d := range devices {
			kind, clients := "Grant", []any{"kubeconfig-generator"}
			if d%2 == 0 {
				kind, clients = "Browser", []any{"console", "grafana"}
			}
			sessions = append(sessions, map[string]any{
				"id": fmt.Sprintf("s%sx%d", userID, d), "kind": kind, "clients": clients,
				"ipAddress": "203.0.113.7", "userAgent": "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0",
				"signedInAt": signedIn, "lastActivity": signedIn, "endsAt": signedIn,
			})
		}
		objs = append(objs,
			&unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "deckhouse.io/v1alpha1", "kind": "UserAccount",
				"metadata": map[string]any{"name": k8s.LocalAccountName(email, userID), "resourceVersion": "1",
					"labels":            map[string]any{"deckhouse.io/user-account-kind": "Local"},
					"managedFields":     []any{map[string]any{"manager": "user-authn-controller", "operation": "Update", "fieldsV1": map[string]any{"f:status": map[string]any{}}}},
					"creationTimestamp": signedIn},
				"status": map[string]any{
					"email": email, "username": userID, "userID": userID, "connectorID": "local", "kind": "Local",
					"groups": []any{"developers", "admins"}, "lastLogin": signedIn,
					"twoFactor": map[string]any{"totp": true}, "sessions": sessions,
				},
			}},
			&unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "dex.coreos.com/v1", "kind": "Password",
				"metadata": map[string]any{"name": "pw-" + userID, "namespace": "d8-user-authn"},
				"email":    email, "username": userID, "userID": userID, "hash": "JDJhJDEwJEUvTWp5ekZpNkdaa3RhOUdIZDh6Q2V1WWlnYkxlblh2MThqa3hPWjZ2aG9Xc0tuYXhOSm91",
			}},
		)
	}
	return objs
}

func newLoadAPIServer(users, devices int) *fake.FakeDynamicClient {
	return fake.NewSimpleDynamicClientWithCustomListKinds(k8sruntime.NewScheme(), map[schema.GroupVersionResource]string{
		{Group: "dex.coreos.com", Version: "v1", Resource: "passwords"}:        "PasswordList",
		{Group: "deckhouse.io", Version: "v1", Resource: "useroperations"}:     "UserOperationList",
		{Group: "deckhouse.io", Version: "v1alpha1", Resource: "useraccounts"}: "UserAccountList",
	}, loadObjects(users, devices)...)
}

func startLoadClient(tb testing.TB, dynamicClient *fake.FakeDynamicClient) *k8s.K8sClient {
	tb.Helper()
	client, err := k8s.NewClientWithDynamic(dynamicClient, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		tb.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := client.Start(ctx); err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(client.Stop)
	return client
}

func BenchmarkSelfDevices(b *testing.B) {
	for _, size := range []struct{ users, devices int }{{1000, 2}, {10000, 2}, {1000, 5}, {10000, 5}, {1000, 20}} {
		b.Run(fmt.Sprintf("U=%d/D=%d", size.users, size.devices), func(b *testing.B) {
			dynamicClient := newLoadAPIServer(size.users, size.devices)
			client := startLoadClient(b, dynamicClient)
			h := newTestHandler(&tokenVerifier{}, client)
			h.readLimiter = newUserLimiter(time.Nanosecond, 1<<30)
			dynamicClient.ClearActions()
			i := 0
			for b.Loop() {
				req := httptest.NewRequest(http.MethodGet, devicesEndpoint, nil)
				req.Header.Set("Authorization", fmt.Sprintf("Bearer user%d", i%size.users))
				w := httptest.NewRecorder()
				h.SelfDevices(w, req)
				if w.Code != http.StatusOK {
					b.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
				}
				i++
			}
			b.ReportMetric(float64(len(dynamicClient.Actions()))/float64(i), "apicalls/op")
		})
	}
}

// TestLoadMemory is the memory the caches of user-api keep for U users with D devices: the heap
// the client adds once synced, over the fake API server that holds the objects.
func TestLoadMemory(t *testing.T) {
	for _, size := range []struct{ users, devices int }{{1000, 2}, {10000, 2}, {10000, 5}, {10000, 20}} {
		dynamicClient := newLoadAPIServer(size.users, size.devices)
		runtime.GC()
		var m0, m1 runtime.MemStats
		runtime.ReadMemStats(&m0)
		client := startLoadClient(t, dynamicClient)
		runtime.GC()
		runtime.ReadMemStats(&m1)
		t.Logf("U=%-6d D=%-3d caches: %.1f MiB, %.0f B per user", size.users, size.devices,
			float64(m1.HeapAlloc-m0.HeapAlloc)/(1<<20), float64(m1.HeapAlloc-m0.HeapAlloc)/float64(size.users))
		runtime.KeepAlive(dynamicClient)
		client.Stop()
	}
}
