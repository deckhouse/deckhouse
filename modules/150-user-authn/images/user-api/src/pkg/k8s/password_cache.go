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
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

const (
	informerResyncPeriod = 5 * time.Minute

	// userIDIndex finds Password objects by the user ID Dex puts into the sub claim of a local
	// user's tokens.
	userIDIndex = "userID"
)

var passwordGVR = schema.GroupVersionResource{
	Group:    "dex.coreos.com",
	Version:  "v1",
	Resource: "passwords",
}

// PasswordCache maps the user IDs of local users to their usernames, from the Dex Password objects.
// It uses a Kubernetes informer to efficiently watch for changes instead of making API calls on
// every request.
type PasswordCache struct {
	informer cache.SharedIndexInformer
	logger   *slog.Logger
	stopCh   chan struct{}
	stopOnce sync.Once
	synced   atomic.Bool
}

// NewPasswordCache creates a new PasswordCache with an informer watching Password objects.
func NewPasswordCache(dynamicClient dynamic.Interface, logger *slog.Logger) (*PasswordCache, error) {
	// Create a filtered dynamic informer factory for the d8-user-authn namespace
	informerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		dynamicClient,
		informerResyncPeriod,
		dexNamespace,
		func(opts *metav1.ListOptions) {},
	)

	informer := informerFactory.ForResource(passwordGVR).Informer()

	// user-api only needs the username of a user ID: the password hashes stay out of its memory.
	if err := informer.SetTransform(trimPassword); err != nil {
		return nil, fmt.Errorf("set password transform: %w", err)
	}
	if err := informer.AddIndexers(cache.Indexers{userIDIndex: passwordUserIDs}); err != nil {
		return nil, fmt.Errorf("add password indexer: %w", err)
	}

	return &PasswordCache{
		informer: informer,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}, nil
}

// Start starts the informer and waits for initial cache sync.
func (pc *PasswordCache) Start(ctx context.Context) error {
	pc.logger.Info("Starting password cache informer")

	go pc.informer.Run(pc.stopCh)

	pc.logger.Info("Waiting for password cache to sync")
	if !cache.WaitForCacheSync(ctx.Done(), pc.informer.HasSynced) {
		// Stop the informer goroutine started above so it doesn't leak.
		pc.Stop()
		return fmt.Errorf("failed to sync password cache")
	}
	pc.synced.Store(true)

	pc.logger.Info("Password cache synced", "local_users_count", pc.Count())
	return nil
}

// Stop stops the informer. It is safe to call multiple times.
func (pc *PasswordCache) Stop() {
	pc.stopOnce.Do(func() {
		close(pc.stopCh)
	})
}

// Username returns the username of the local user with the given user ID.
func (pc *PasswordCache) Username(userID string) (string, error) {
	if !pc.IsSynced() {
		return "", ErrCacheNotSynced
	}

	objs, err := pc.informer.GetIndexer().ByIndex(userIDIndex, userID)
	if err != nil {
		return "", fmt.Errorf("look up password by user ID: %w", err)
	}

	switch len(objs) {
	case 0:
		return "", ErrUserNotFound
	case 1:
	default:
		// Two Password objects with one user ID give their users the same tokens. Which of them
		// signed in cannot be told, so none of them is acted upon.
		return "", ErrAmbiguousUser
	}

	username := passwordField(objs[0], "username")
	if username == "" {
		return "", ErrUserNotFound
	}
	return username, nil
}

// IsSynced returns true if the cache has completed initial sync.
func (pc *PasswordCache) IsSynced() bool {
	return pc.synced.Load()
}

// Count returns the number of local users in the cache.
func (pc *PasswordCache) Count() int {
	return len(pc.informer.GetStore().ListKeys())
}

// passwordUserIDs indexes a Password object by the user ID of its user. The User controller writes
// the username as the user ID, and the UserOperation controller takes the username for an object
// without one; the index does the same.
func passwordUserIDs(obj any) ([]string, error) {
	userID := passwordField(obj, "userID")
	if userID == "" {
		userID = passwordField(obj, "username")
	}
	if userID == "" {
		return nil, nil
	}
	return []string{userID}, nil
}

// trimPassword keeps only the identity of a Password object: its name, username and user ID.
func trimPassword(obj any) (any, error) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return obj, nil
	}

	trimmed := &unstructured.Unstructured{Object: map[string]any{}}
	trimmed.SetAPIVersion(u.GetAPIVersion())
	trimmed.SetKind(u.GetKind())
	trimmed.SetNamespace(u.GetNamespace())
	trimmed.SetName(u.GetName())
	trimmed.SetUID(u.GetUID())
	trimmed.SetResourceVersion(u.GetResourceVersion())
	for _, field := range []string{"username", "userID"} {
		if value, found, err := unstructured.NestedString(u.Object, field); err == nil && found {
			trimmed.Object[field] = value
		}
	}

	return trimmed, nil
}

func passwordField(obj any, field string) string {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return ""
	}

	value, found, err := unstructured.NestedString(u.Object, field)
	if err != nil || !found {
		return ""
	}
	return value
}
