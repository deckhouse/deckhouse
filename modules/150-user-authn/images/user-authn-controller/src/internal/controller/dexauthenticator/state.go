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

package dexauthenticator

import (
	"cmp"
	"maps"
	"reflect"
	"slices"
	"sync"

	"k8s.io/apimachinery/pkg/types"

	"user-authn-controller/internal/naming"
)

// State is what the table reconciler decided about every DexAuthenticator, kept for the
// reconciler of their objects: reading the table Secret on every reconcile of every object would
// read the whole table hundreds of times in a row.
type State struct {
	mu      sync.RWMutex
	ready   bool
	entries map[types.NamespacedName]Entry
	// platform are the objects of the d8-* namespaces: the clients that share one sign-in.
	platform []types.NamespacedName
}

// NewState returns a State that knows nothing yet.
func NewState() *State {
	return &State{entries: map[types.NamespacedName]Entry{}}
}

// Entry returns the decision about the object key, whether the table reconciler has seen the
// object, and whether it has made its first pass.
func (s *State) Entry(key types.NamespacedName) (Entry, bool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, known := s.entries[key]
	return entry, known, s.ready
}

// Platform returns the objects of the d8-* namespaces.
func (s *State) Platform() []types.NamespacedName {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return slices.Clone(s.platform)
}

// set replaces the decisions and returns the objects whose reconcile they change: those with
// another decision, and every object of the d8-* namespaces when their set changed, since each of
// them names the others in its Dex client.
func (s *State) set(entries map[types.NamespacedName]Entry) []types.NamespacedName {
	var platform []types.NamespacedName
	for key := range entries {
		if naming.IsSystemNamespace(key.Namespace) {
			platform = append(platform, key)
		}
	}
	slices.SortFunc(platform, compareKeys)

	s.mu.Lock()
	defer s.mu.Unlock()

	changed := make(map[types.NamespacedName]struct{})
	for key, entry := range entries {
		if old, ok := s.entries[key]; !ok || !s.ready || !reflect.DeepEqual(old, entry) {
			changed[key] = struct{}{}
		}
	}
	for key := range s.entries {
		if _, still := entries[key]; !still {
			// The object is gone: its reconcile removes what is left of it.
			changed[key] = struct{}{}
		}
	}
	if !slices.Equal(platform, s.platform) {
		for _, key := range platform {
			changed[key] = struct{}{}
		}
		for _, key := range s.platform {
			changed[key] = struct{}{}
		}
	}

	s.entries, s.platform, s.ready = entries, platform, true
	return slices.SortedFunc(maps.Keys(changed), compareKeys)
}

func compareKeys(a, b types.NamespacedName) int {
	return cmp.Or(cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Name, b.Name))
}
