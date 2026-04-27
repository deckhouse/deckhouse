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
)

// PasswordCache maintains a cached set of local usernames from Password CRDs.
// It uses a Kubernetes informer to efficiently watch for changes instead of
// making API calls on every request.
type PasswordCache struct {
	mu            sync.RWMutex
	localUsers    map[string]struct{}
	informer      cache.SharedIndexInformer
	logger        *slog.Logger
	stopCh        chan struct{}
	synced        bool
	dynamicClient dynamic.Interface
}

// NewPasswordCache creates a new PasswordCache with an informer watching Password CRDs.
func NewPasswordCache(dynamicClient dynamic.Interface, logger *slog.Logger) *PasswordCache {
	pc := &PasswordCache{
		localUsers:    make(map[string]struct{}),
		logger:        logger,
		stopCh:        make(chan struct{}),
		dynamicClient: dynamicClient,
	}

	// Create a filtered dynamic informer factory for the d8-user-authn namespace
	informerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		dynamicClient,
		informerResyncPeriod,
		dexNamespace,
		func(opts *metav1.ListOptions) {},
	)

	gvr := schema.GroupVersionResource{
		Group:    "dex.coreos.com",
		Version:  "v1",
		Resource: "passwords",
	}

	pc.informer = informerFactory.ForResource(gvr).Informer()

	_, _ = pc.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.onAdd,
		UpdateFunc: pc.onUpdate,
		DeleteFunc: pc.onDelete,
	})

	return pc
}

// Start starts the informer and waits for initial cache sync.
func (pc *PasswordCache) Start(ctx context.Context) error {
	pc.logger.Info("Starting password cache informer")

	go pc.informer.Run(pc.stopCh)

	pc.logger.Info("Waiting for password cache to sync")
	if !cache.WaitForCacheSync(ctx.Done(), pc.informer.HasSynced) {
		return fmt.Errorf("failed to sync password cache")
	}

	pc.mu.Lock()
	pc.synced = true
	pc.mu.Unlock()

	pc.logger.Info("Password cache synced", "local_users_count", pc.Count())
	return nil
}

// Stop stops the informer.
func (pc *PasswordCache) Stop() {
	close(pc.stopCh)
}

// IsLocalUser checks if the given username exists in the cache.
func (pc *PasswordCache) IsLocalUser(username string) bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	_, exists := pc.localUsers[username]
	return exists
}

// IsSynced returns true if the cache has completed initial sync.
func (pc *PasswordCache) IsSynced() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.synced
}

// Count returns the number of local users in the cache.
func (pc *PasswordCache) Count() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return len(pc.localUsers)
}

func (pc *PasswordCache) onAdd(obj interface{}) {
	username := pc.extractUsername(obj)
	if username == "" {
		return
	}

	pc.mu.Lock()
	pc.localUsers[username] = struct{}{}
	pc.mu.Unlock()

	pc.logger.Debug("Added local user to cache", "username", username)
}

func (pc *PasswordCache) onUpdate(oldObj, newObj interface{}) {
	oldUsername := pc.extractUsername(oldObj)
	newUsername := pc.extractUsername(newObj)

	if oldUsername == newUsername {
		return
	}

	pc.mu.Lock()
	if oldUsername != "" {
		delete(pc.localUsers, oldUsername)
	}
	if newUsername != "" {
		pc.localUsers[newUsername] = struct{}{}
	}
	pc.mu.Unlock()

	pc.logger.Debug("Updated local user in cache", "old_username", oldUsername, "new_username", newUsername)
}

func (pc *PasswordCache) onDelete(obj interface{}) {
	// Handle DeletedFinalStateUnknown (when the watch connection is lost and
	// we miss the delete event)
	if deletedState, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = deletedState.Obj
	}

	username := pc.extractUsername(obj)
	if username == "" {
		return
	}

	pc.mu.Lock()
	delete(pc.localUsers, username)
	pc.mu.Unlock()

	pc.logger.Debug("Deleted local user from cache", "username", username)
}

func (pc *PasswordCache) extractUsername(obj interface{}) string {
	unstr, ok := obj.(*unstructured.Unstructured)
	if !ok {
		pc.logger.Warn("Failed to convert object to unstructured")
		return ""
	}

	username, found, err := unstructured.NestedString(unstr.Object, "username")
	if err != nil || !found {
		return ""
	}

	return username
}
