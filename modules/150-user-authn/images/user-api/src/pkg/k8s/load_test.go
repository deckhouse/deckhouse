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

package k8s

// The memory the caches of user-api keep in steady state: the stores of the Password and UserAccount
// informers, with their transforms and indexes, for U users with D devices. Run it with
//
//	go test -tags load -run TestLoadCacheMemory -v ./pkg/k8s/

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

func loadAccount(i, devices int) *unstructured.Unstructured {
	userID := fmt.Sprintf("user%d", i)
	signedIn := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC).Format(time.RFC3339)
	sessions := make([]any, 0, devices)
	for d := range devices {
		kind, clients := "Grant", []any{"kubeconfig-generator"}
		if d%2 == 0 {
			kind, clients = "Browser", []any{"console", "grafana"}
		}
		sessions = append(sessions, map[string]any{
			"id": fmt.Sprintf("%026d", i*100+d), "kind": kind, "clients": clients,
			"ipAddress":  fmt.Sprintf("10.%d.%d.%d", i/65536%256, i/256%256, i%256),
			"userAgent":  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
			"signedInAt": signedIn, "knownSince": signedIn, "lastActivity": signedIn, "endsAt": signedIn,
		})
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1", "kind": "UserAccount",
		"metadata": map[string]any{"name": LocalAccountName(userID+"@example.com", userID), "resourceVersion": "12345", "uid": fmt.Sprintf("%036d", i)},
		"status": map[string]any{
			"email": userID + "@example.com", "userID": userID, "connectorID": "local", "sessions": sessions,
		},
	}}
}

func loadPassword(i int) *unstructured.Unstructured {
	userID := fmt.Sprintf("user%d", i)
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "dex.coreos.com/v1", "kind": "Password",
		"metadata": map[string]any{"name": fmt.Sprintf("pw%038d", i), "namespace": dexNamespace, "resourceVersion": "12345", "uid": fmt.Sprintf("%036d", i)},
		"email":    userID + "@example.com", "username": userID, "userID": userID, "hash": "JDJhJDEwJEUvTWp5ekZpNkdaa3RhOUdIZDh6Q2V1WWlnYkxlblh2MThqa3hPWjZ2aG9Xc0tuYXhOSm91",
	}}
}

func TestLoadCacheMemory(t *testing.T) {
	ac := &AccountCache{logger: testLogger()}
	for _, size := range []struct{ users, devices int }{{1000, 2}, {10000, 2}, {10000, 5}, {10000, 20}, {10000, 50}} {
		runtime.GC()
		var m0, m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m0)
		accounts := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{pairIndex: accountPairs})
		for i := range size.users {
			obj, _ := ac.trimAccount(loadAccount(i, size.devices))
			_ = accounts.Add(obj)
		}
		runtime.GC()
		runtime.ReadMemStats(&m1)
		passwords := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{userIDIndex: passwordUserIDs})
		for i := range size.users {
			obj, _ := trimPassword(loadPassword(i))
			_ = passwords.Add(obj)
		}
		runtime.GC()
		runtime.ReadMemStats(&m2)
		acc, pw := float64(m1.HeapAlloc-m0.HeapAlloc), float64(m2.HeapAlloc-m1.HeapAlloc)
		t.Logf("U=%-6d D=%-3d accounts %.1f MiB (%.0f B/user), passwords %.1f MiB (%.0f B/user), total %.1f MiB",
			size.users, size.devices, acc/(1<<20), acc/float64(size.users), pw/(1<<20), pw/float64(size.users), (acc+pw)/(1<<20))
		runtime.KeepAlive(accounts)
		runtime.KeepAlive(passwords)
	}
}
