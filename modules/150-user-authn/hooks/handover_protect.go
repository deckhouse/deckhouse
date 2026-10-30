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
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/modules/150-user-authn/hooks/internal/handover"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        handoverQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
}, dependency.WithExternalDependencies(protectHandoverObjects))

// handedOver is set once a run finds no object left to protect: the chart this deckhouse runs
// renders none, and the controller keeps the policy on the Passwords, so the later runs have
// nothing to do and make no requests. A deckhouse of another version is another process and
// checks again.
var handedOver atomic.Bool

func protectHandoverObjects(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	if handedOver.Load() {
		return nil
	}
	c, err := newHandoverClient(dc)
	if err != nil {
		return fmt.Errorf("build client: %w", err)
	}
	res, err := handover.Protect(ctx, c, handoverKinds)
	if res.Stamped > 0 {
		input.Logger.Info("stamped keep policy on objects handed over to user-authn-controller", slog.Int("count", res.Stamped))
	}
	if err != nil {
		return fmt.Errorf("protect objects handed over to user-authn-controller: %w", err)
	}
	if res.Owned == 0 {
		handedOver.Store(true)
	}
	return nil
}
