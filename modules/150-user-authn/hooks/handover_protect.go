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

package hooks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/modules/150-user-authn/hooks/internal/handover"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        handoverQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
}, dependency.WithExternalDependencies(protectHandoverObjects))

// handedOver remembers the kinds a run found no object of that the release owns: the controller
// has adopted them all, and the chart this deckhouse runs renders none, so the later runs have
// nothing of the kind to protect and make no request for it. A deckhouse of another version is
// another process and checks again.
//
// The kinds are remembered one by one. An object that stays on its own dex-authenticator keeps the
// objects of that dex-authenticator in the release for as long as it does, which may be for good;
// were the hook to stop only when nothing at all is left, every run of the module would go on
// listing the metadata of every Secret of the cluster for the DexClients, whose objects were
// adopted long ago.
//
// TODO: remove the handover hooks and package handover once no supported update path starts from a
// chart that renders any of the kinds: the objects of DexClients and the Dex passwords moved to the
// controller first, the dex-authenticator of every DexAuthenticator with the shared instances.
var handedOver handedOverKinds

// handedOverKinds are the names of the kinds that have nothing left to protect.
type handedOverKinds struct {
	mu    sync.Mutex
	names map[string]struct{}
}

// pending returns the kinds of kinds that may still have an object to protect.
func (h *handedOverKinds) pending(kinds []handover.Kind) []handover.Kind {
	h.mu.Lock()
	defer h.mu.Unlock()

	out := make([]handover.Kind, 0, len(kinds))
	for _, kind := range kinds {
		if _, done := h.names[kind.Name]; !done {
			out = append(out, kind)
		}
	}
	return out
}

func (h *handedOverKinds) done(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.names == nil {
		h.names = map[string]struct{}{}
	}
	h.names[name] = struct{}{}
}

func protectHandoverObjects(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	kinds := handedOver.pending(handoverKinds)
	if len(kinds) == 0 {
		return nil
	}
	c, err := newHandoverClient(dc)
	if err != nil {
		return fmt.Errorf("build client: %w", err)
	}

	var stamped int
	var errs []error
	for _, kind := range kinds {
		res, err := handover.Protect(ctx, c, []handover.Kind{kind})
		stamped += res.Stamped
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if res.Owned == 0 {
			handedOver.done(kind.Name)
		}
	}
	if stamped > 0 {
		input.Logger.Info("stamped keep policy on objects handed over to user-authn-controller", slog.Int("count", stamped))
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("protect objects handed over to user-authn-controller: %w", err)
	}
	return nil
}
