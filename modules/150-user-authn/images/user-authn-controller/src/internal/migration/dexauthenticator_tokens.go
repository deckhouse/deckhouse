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

// Package migration converts state that earlier versions of the module left in the cluster into
// the form the current version uses. README.md describes every migration.
package migration

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// defaultRetryInterval is how often DexAuthenticatorTokens checks again while the OAuth2Client of a
// legacy client id is still there. A check reads the cache only: the DexAuthenticators by metadata
// and the OAuth2Clients by the index of their id.
const defaultRetryInterval = 30 * time.Second

// LegacyDexAuthenticatorClientID is the id the dex client of a DexAuthenticator had before it
// became dex-authenticator-<name>@<namespace>.
func LegacyDexAuthenticatorClientID(name, namespace string) string {
	return fmt.Sprintf("%s-%s-dex-authenticator", name, namespace)
}

// DexAuthenticatorTokens deletes what Dex keeps for the dex clients of DexAuthenticators under
// their legacy ids: the refresh tokens, the references to them in OfflineSessions (refresh and
// grants), and the client states of browser sessions. The chart replaced these clients, so Dex
// refuses the tokens, but its GC deletes a refresh token only after it has gone unused for
// refreshTokenValidIfNotUsedFor, and UserAccount lists them as grants until then.
//
// It touches an id only when no OAuth2Client has that id any more: until Helm prunes the old
// client the id still belongs to it, and an id that some other client happens to have is never
// touched. It runs on the leader at start and checks again every retry interval until no legacy id
// has an OAuth2Client; an id that another client keeps for good (DexAuthenticator kubeconfig in foo
// and the kubeconfig cluster foo-dex-authenticator share kubeconfig-foo-dex-authenticator) keeps it
// checking, which costs reads of the cache only. The Dex objects are scanned only when an id becomes
// free that this process has not cleaned yet. Every step deletes or removes only what is still
// there, so a restart halfway runs the rest.
type DexAuthenticatorTokens struct {
	client   client.Client
	log      logr.Logger
	interval time.Duration
	// cleaned holds the legacy ids whose Dex objects this process has already deleted.
	cleaned map[string]struct{}
}

// NewDexAuthenticatorTokens returns the migration; interval 0 means the default retry interval.
func NewDexAuthenticatorTokens(c client.Client, log logr.Logger, interval time.Duration) *DexAuthenticatorTokens {
	if interval <= 0 {
		interval = defaultRetryInterval
	}
	return &DexAuthenticatorTokens{client: c, log: log, interval: interval, cleaned: map[string]struct{}{}}
}

// Register adds the migrations to the manager.
func Register(mgr manager.Manager) error {
	m := NewDexAuthenticatorTokens(mgr.GetClient(), mgr.GetLogger().WithName("migration").WithName("dex-authenticator-tokens"), 0)
	if err := mgr.Add(m); err != nil {
		return fmt.Errorf("add dex-authenticator-tokens migration: %w", err)
	}
	return nil
}

// NeedLeaderElection makes the migration run on the leader only.
func (m *DexAuthenticatorTokens) NeedLeaderElection() bool {
	return true
}

// Start runs the migration until no legacy id is left with an OAuth2Client, or ctx ends.
func (m *DexAuthenticatorTokens) Start(ctx context.Context) error {
	waiting := false
	for {
		pending, err := m.Run(ctx)
		switch {
		case err != nil && ctx.Err() == nil:
			m.log.Error(err, "Failed to delete the refresh tokens of the legacy DexAuthenticator client ids")
		case err == nil && len(pending) == 0:
			return nil
		case err == nil && !waiting:
			m.log.Info("Waiting for the OAuth2Clients of the legacy DexAuthenticator client ids to be deleted", "clientIDs", pending)
			waiting = true
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(m.interval):
		}
	}
}

// Run makes one pass. It returns the legacy ids it left alone because an OAuth2Client still has
// them. The Dex objects are listed only when a legacy id is free that the process has not cleaned.
func (m *DexAuthenticatorTokens) Run(ctx context.Context) ([]string, error) {
	ids, pending, err := m.legacyIDs(ctx)
	if err != nil {
		return pending, err
	}
	for id := range ids {
		if _, done := m.cleaned[id]; done {
			delete(ids, id)
		}
	}
	if len(ids) == 0 {
		return pending, nil
	}

	tokens, err := m.deleteTokens(ctx, ids)
	if err != nil {
		return pending, err
	}
	refs, err := m.removeRefs(ctx, ids)
	if err != nil {
		return pending, err
	}
	states, err := m.removeClientStates(ctx, ids)
	if err != nil {
		return pending, err
	}
	for id := range ids {
		m.cleaned[id] = struct{}{}
	}
	if tokens+refs+states > 0 {
		m.log.Info("Deleted the refresh tokens of the legacy DexAuthenticator client ids",
			"refreshTokensDeleted", tokens,
			"offlineSessionsReferencesRemoved", refs,
			"authSessionClientStatesRemoved", states,
		)
	}
	return pending, nil
}

// legacyIDs returns the legacy ids of the DexAuthenticators that no OAuth2Client has (ids) and those
// one still has (pending).
func (m *DexAuthenticatorTokens) legacyIDs(ctx context.Context) (map[string]struct{}, []string, error) {
	authenticators := &metav1.PartialObjectMetadataList{}
	authenticators.SetGroupVersionKind(controller.DexAuthenticatorGVK.GroupVersion().WithKind(controller.DexAuthenticatorGVK.Kind + "List"))
	if err := m.client.List(ctx, authenticators); err != nil {
		return nil, nil, fmt.Errorf("list DexAuthenticators: %w", err)
	}
	ids := map[string]struct{}{}
	var pending []string
	for _, a := range authenticators.Items {
		id := LegacyDexAuthenticatorClientID(a.Name, a.Namespace)
		if _, seen := ids[id]; seen || slices.Contains(pending, id) {
			continue
		}
		clients := controller.List(controller.OAuth2ClientGVK)
		if err := m.client.List(ctx, clients, client.InNamespace(naming.DexNamespace), client.MatchingFields{controller.OAuth2ClientIDField: id}); err != nil {
			return nil, nil, fmt.Errorf("list OAuth2Clients of %s: %w", id, err)
		}
		if len(clients.Items) > 0 {
			pending = append(pending, id)
			continue
		}
		ids[id] = struct{}{}
	}
	slices.Sort(pending)
	return ids, pending, nil
}

// deleteTokens deletes the RefreshToken objects issued to the ids.
func (m *DexAuthenticatorTokens) deleteTokens(ctx context.Context, ids map[string]struct{}) (int, error) {
	list := controller.List(controller.RefreshTokenGVK)
	if err := m.client.List(ctx, list, client.InNamespace(naming.DexNamespace)); err != nil {
		return 0, fmt.Errorf("list refresh tokens: %w", err)
	}
	deleted := 0
	for i := range list.Items {
		item := &list.Items[i]
		clientID, _, _ := unstructured.NestedString(item.Object, "clientID")
		if _, legacy := ids[clientID]; !legacy {
			continue
		}
		if err := m.client.Delete(ctx, item); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return deleted, fmt.Errorf("delete refresh token %s: %w", item.GetName(), err)
		}
		deleted++
	}
	return deleted, nil
}

// removeRefs removes the references of OfflineSessions to grants of the ids: the entry of the
// client in refresh, and the entries of grants that name the client. Each removal tests that the
// entry still names the same refresh token, so a sign-in in between keeps what it wrote.
func (m *DexAuthenticatorTokens) removeRefs(ctx context.Context, ids map[string]struct{}) (int, error) {
	list := controller.List(controller.OfflineSessionsGVK)
	if err := m.client.List(ctx, list, client.InNamespace(naming.DexNamespace)); err != nil {
		return 0, fmt.Errorf("list offlinesessions: %w", err)
	}
	removed := 0
	for i := range list.Items {
		item := &list.Items[i]
		refresh, _, _ := unstructured.NestedMap(item.Object, "refresh")
		for _, key := range sortedKeys(refresh) {
			if _, legacy := ids[key]; !legacy {
				continue
			}
			ok, err := m.removeEntry(ctx, item.GetName(), "/refresh/"+escapePointer(key), refresh[key])
			if err != nil {
				return removed, err
			}
			if ok {
				removed++
			}
		}
		grants, _, _ := unstructured.NestedMap(item.Object, "grants")
		for _, key := range sortedKeys(grants) {
			entry, _ := grants[key].(map[string]any)
			clientID, _ := entry["ClientID"].(string)
			if _, legacy := ids[clientID]; !legacy {
				continue
			}
			ok, err := m.removeEntry(ctx, item.GetName(), "/grants/"+escapePointer(key), grants[key])
			if err != nil {
				return removed, err
			}
			if ok {
				removed++
			}
		}
	}
	return removed, nil
}

// removeEntry removes the entry at path of the OfflineSessions name if it still names the refresh
// token it named in the cache. It reports whether the entry was removed.
func (m *DexAuthenticatorTokens) removeEntry(ctx context.Context, name, path string, raw any) (bool, error) {
	entry, _ := raw.(map[string]any)
	id, _ := entry["ID"].(string)
	var patch string
	if id != "" {
		patch = fmt.Sprintf(`[{"op":"test","path":%q,"value":%q},{"op":"remove","path":%q}]`, path+"/ID", id, path)
	} else {
		patch = fmt.Sprintf(`[{"op":"remove","path":%q}]`, path)
	}
	return m.patch(ctx, controller.OfflineSessionsGVK, name, patch)
}

// removeClientStates removes the ids from the client states of browser sessions, so a session no
// longer lists the replaced client among its applications.
func (m *DexAuthenticatorTokens) removeClientStates(ctx context.Context, ids map[string]struct{}) (int, error) {
	list := controller.List(controller.AuthSessionGVK)
	if err := m.client.List(ctx, list, client.InNamespace(naming.DexNamespace)); err != nil {
		return 0, fmt.Errorf("list auth sessions: %w", err)
	}
	removed := 0
	for i := range list.Items {
		item := &list.Items[i]
		states, _, _ := unstructured.NestedMap(item.Object, "clientStates")
		for _, key := range sortedKeys(states) {
			if _, legacy := ids[key]; !legacy {
				continue
			}
			ok, err := m.patch(ctx, controller.AuthSessionGVK, item.GetName(), fmt.Sprintf(`[{"op":"remove","path":%q}]`, "/clientStates/"+escapePointer(key)))
			if err != nil {
				return removed, err
			}
			if ok {
				removed++
			}
		}
	}
	return removed, nil
}

// patch applies a JSON Patch to a Dex object. A patch that no longer applies (the test failed, the
// path or the object is gone) means somebody else changed it first; it is not an error.
func (m *DexAuthenticatorTokens) patch(ctx context.Context, gvk schema.GroupVersionKind, name, patch string) (bool, error) {
	obj := controller.Object(gvk)
	obj.SetNamespace(naming.DexNamespace)
	obj.SetName(name)
	err := m.client.Patch(ctx, obj, client.RawPatch(types.JSONPatchType, []byte(patch)))
	switch {
	case err == nil:
		return true, nil
	case apierrors.IsNotFound(err), apierrors.IsInvalid(err):
		return false, nil
	default:
		return false, fmt.Errorf("patch %s %s: %w", gvk.Kind, name, err)
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// escapePointer escapes a key for a JSON Pointer (RFC 6901).
func escapePointer(s string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(s)
}
