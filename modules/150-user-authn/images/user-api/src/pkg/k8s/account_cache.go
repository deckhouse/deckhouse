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
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"unique"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// AccountCache keeps the devices of every UserAccount, from an informer indexed by the pair of the
// account: a request reads them from memory, not from the API server.
type AccountCache struct {
	informer cache.SharedIndexInformer
	logger   *slog.Logger
	stopCh   chan struct{}
	stopOnce sync.Once
	synced   atomic.Bool
}

// cachedAccount is what AccountCache keeps of a UserAccount: its name and devices. The email, the
// groups and the rest of the status stay out of memory.
type cachedAccount struct {
	metav1.ObjectMeta
	devices Devices
}

// pairIndex finds a cached UserAccount by its pair, pairKey of status.connectorID and status.userID.
const pairIndex = "pair"

func pairKey(userID, connectorID string) string { return connectorID + "/" + userID }

// NewAccountCache creates an AccountCache with an informer watching UserAccount objects. Every
// UserAccount is watched: any user may ask for their devices, and the labels tell no account apart.
func NewAccountCache(dynamicClient dynamic.Interface, logger *slog.Logger) (*AccountCache, error) {
	// No resync: nothing handles the events, the store is read on request.
	ac := &AccountCache{
		informer: dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0).ForResource(userAccountGVR).Informer(),
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
	if err := ac.informer.SetTransform(ac.trimAccount); err != nil {
		return nil, fmt.Errorf("set user account transform: %w", err)
	}
	if err := ac.informer.AddIndexers(cache.Indexers{pairIndex: accountPairs}); err != nil {
		return nil, fmt.Errorf("add user account indexer: %w", err)
	}
	return ac, nil
}

// accountPairs indexes a cached UserAccount by its pair; one without a pair yet is not indexed.
func accountPairs(obj any) ([]string, error) {
	account, ok := obj.(*cachedAccount)
	if !ok || account.devices.UserID == "" || account.devices.ConnectorID == "" {
		return nil, nil
	}
	return []string{pairKey(account.devices.UserID, account.devices.ConnectorID)}, nil
}

// Start starts the informer and waits for initial cache sync.
func (ac *AccountCache) Start(ctx context.Context) error {
	ac.logger.Info("Starting user account cache informer")
	go ac.informer.Run(ac.stopCh)

	if !cache.WaitForCacheSync(ctx.Done(), ac.informer.HasSynced) {
		ac.Stop()
		return fmt.Errorf("failed to sync user account cache")
	}
	ac.synced.Store(true)
	ac.logger.Info("User account cache synced", "user_accounts_count", len(ac.informer.GetStore().ListKeys()))
	return nil
}

// Stop stops the informer. It is safe to call multiple times.
func (ac *AccountCache) Stop() {
	ac.stopOnce.Do(func() {
		close(ac.stopCh)
	})
}

// IsSynced tells whether the cache has completed its initial sync.
func (ac *AccountCache) IsSynced() bool {
	return ac.synced.Load()
}

// Devices returns the devices of the UserAccount of the pair (userID, connectorID); none when there
// is no such object.
func (ac *AccountCache) Devices(userID, connectorID string) (Devices, error) {
	if !ac.synced.Load() {
		return Devices{}, ErrCacheNotSynced
	}
	objs, err := ac.informer.GetIndexer().ByIndex(pairIndex, pairKey(userID, connectorID))
	if err != nil {
		return Devices{}, fmt.Errorf("look up the user account of %s: %w", pairKey(userID, connectorID), err)
	}
	switch len(objs) {
	case 0:
		return Devices{}, nil
	case 1:
	default:
		// The controller keeps one UserAccount per pair; two would mix the devices of two users.
		return Devices{}, fmt.Errorf("%w: %d user accounts of %s", ErrAmbiguousUser, len(objs), pairKey(userID, connectorID))
	}
	account, ok := objs[0].(*cachedAccount)
	if !ok {
		return Devices{}, fmt.Errorf("the user account of %s is cached as %T", pairKey(userID, connectorID), objs[0])
	}
	// The cached object is shared with the informer; the caller gets a list of its own.
	devices := account.devices
	devices.Items = slices.Clone(devices.Items)
	return devices, nil
}

// trimAccount keeps only the name and the devices of a UserAccount (cachedAccount). A status that
// cannot be read leaves the account without devices, as if it had none yet. The transform has to be
// idempotent: client-go may run it again on an object it already trimmed.
func (ac *AccountCache) trimAccount(obj any) (any, error) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return obj, nil
	}
	trimmed := &cachedAccount{ObjectMeta: metav1.ObjectMeta{
		Name:            u.GetName(),
		UID:             u.GetUID(),
		ResourceVersion: u.GetResourceVersion(),
	}}
	devices, err := decodeDevices(u)
	if err != nil {
		ac.logger.Warn("Failed to read the devices of a user account", "user_account", u.GetName(), "error", err)
		return trimmed, nil
	}
	trimmed.devices = devices
	return trimmed, nil
}

// decodeDevices reads status.sessions of a UserAccount and the pair it belongs to.
func decodeDevices(account *unstructured.Unstructured) (Devices, error) {
	var status struct {
		UserID            string   `json:"userID"`
		ConnectorID       string   `json:"connectorID"`
		Sessions          []Device `json:"sessions"`
		SessionsTruncated bool     `json:"sessionsTruncated"`
	}
	raw, found, err := unstructured.NestedFieldNoCopy(account.Object, "status")
	if err != nil || !found {
		return Devices{}, err
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return Devices{}, fmt.Errorf("encode the status of user account %s: %w", account.GetName(), err)
	}
	if err := json.Unmarshal(data, &status); err != nil {
		return Devices{}, fmt.Errorf("decode the status of user account %s: %w", account.GetName(), err)
	}
	// The decoder grows the list as it goes; the cache keeps a list of the exact length.
	items := make([]Device, len(status.Sessions))
	copy(items, status.Sessions)
	for i := range items {
		internDevice(&items[i])
	}
	return Devices{
		Items:       items,
		Truncated:   status.SessionsTruncated,
		UserID:      status.UserID,
		ConnectorID: status.ConnectorID,
	}, nil
}

// internDevice makes the strings of d that repeat across devices share their memory: the kind, the
// applications and the User-Agent, which is the same for every device of one browser version.
func internDevice(d *Device) {
	d.Kind = unique.Make(d.Kind).Value()
	d.UserAgent = unique.Make(d.UserAgent).Value()
	for i, c := range d.Clients {
		d.Clients[i] = unique.Make(c).Value()
	}
}
