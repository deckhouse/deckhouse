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

package useridentity

import (
	"context"
	"fmt"
	"slices"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// fieldKnownDevices holds the browsers that completed a login of the pair: Dex adds the hash of the
// device mark of the browser with the time it was first and last seen.
const fieldKnownDevices = "knownDevices"

// KnownDevices returns when Dex first saw each browser of the pair complete a login, by the hash of
// its device mark. An entry that does not read is skipped: a device of unknown age is new.
func (i *Identity) KnownDevices() map[string]time.Time {
	raw, ok := i.obj.Object[fieldKnownDevices].([]any)
	if !ok {
		return nil
	}
	known := make(map[string]time.Time, len(raw))
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		hash := stringField(entry, "hash")
		firstSeen, err := time.Parse(time.RFC3339Nano, stringField(entry, "firstSeen"))
		if hash == "" || err != nil || firstSeen.Year() <= 1 {
			continue
		}
		known[hash] = firstSeen
	}
	return known
}

// ForgetKnownDevices removes the browsers hashes from the known devices and reports whether any was
// there; with no hashes it removes every browser.
func (i *Identity) ForgetKnownDevices(hashes ...string) bool {
	raw, ok := i.obj.Object[fieldKnownDevices].([]any)
	if !ok || len(raw) == 0 {
		return false
	}
	if len(hashes) == 0 {
		delete(i.obj.Object, fieldKnownDevices)
		return true
	}
	kept := make([]any, 0, len(raw))
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		if ok && slices.Contains(hashes, stringField(entry, "hash")) {
			continue
		}
		kept = append(kept, item)
	}
	if len(kept) == len(raw) {
		return false
	}
	if len(kept) == 0 {
		delete(i.obj.Object, fieldKnownDevices)
		return true
	}
	i.obj.Object[fieldKnownDevices] = kept
	return true
}

// KeepOnlyKnownDevice removes every browser but keep from the known devices and reports whether any
// was removed.
func (i *Identity) KeepOnlyKnownDevice(keep string) bool {
	raw, ok := i.obj.Object[fieldKnownDevices].([]any)
	if !ok || len(raw) == 0 {
		return false
	}
	kept := make([]any, 0, 1)
	for _, item := range raw {
		if entry, ok := item.(map[string]any); ok && keep != "" && stringField(entry, "hash") == keep {
			kept = append(kept, item)
		}
	}
	if len(kept) == len(raw) {
		return false
	}
	if len(kept) == 0 {
		delete(i.obj.Object, fieldKnownDevices)
		return true
	}
	i.obj.Object[fieldKnownDevices] = kept
	return true
}

// ForgetOtherDevices removes every browser but keep from the known devices of the pair: the user
// signed out of every other device, so none of them counts as known any more, whether it is signed
// in now or not. reader must bypass the cache.
func ForgetOtherDevices(ctx context.Context, reader client.Reader, writer client.Writer, pair Pair, keep string) error {
	_, err := Mutate(ctx, reader, writer, pair.Name(), func(current *Identity) (*Identity, error) {
		if current == nil || current.UserID() != pair.UserID || current.ConnectorID() != pair.ConnectorID {
			return nil, nil
		}
		if !current.KeepOnlyKnownDevice(keep) {
			return nil, nil
		}
		return current, nil
	})
	if err != nil {
		return fmt.Errorf("forget the other devices of useridentity %s: %w", pair.Name(), err)
	}
	return nil
}

// ForgetDevices removes the browsers hashes from the known devices of the pair: a sign-in of each was
// ended by another sign-in or by an administrator, so its next login counts as its first. reader must
// bypass the cache. A pair without an identity, or without these browsers, is left as it is.
func ForgetDevices(ctx context.Context, reader client.Reader, writer client.Writer, pair Pair, hashes []string) error {
	hashes = slices.DeleteFunc(slices.Clone(hashes), func(h string) bool { return h == "" })
	if len(hashes) == 0 {
		return nil
	}
	_, err := Mutate(ctx, reader, writer, pair.Name(), func(current *Identity) (*Identity, error) {
		if current == nil || current.UserID() != pair.UserID || current.ConnectorID() != pair.ConnectorID {
			return nil, nil
		}
		if !current.ForgetKnownDevices(hashes...) {
			return nil, nil
		}
		return current, nil
	})
	if err != nil {
		return fmt.Errorf("forget devices of useridentity %s: %w", pair.Name(), err)
	}
	return nil
}
