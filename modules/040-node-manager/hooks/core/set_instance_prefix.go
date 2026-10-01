/*
Copyright 2021 Flant JSC

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

package core

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/instanceprefix"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 100},
}, handleSetInstancePrefix)

// The value is published for templates only: they render after every beforeHelm hook. Hooks call
// instanceprefix.Resolve themselves, since they may run before this one.
func handleSetInstancePrefix(_ context.Context, input *go_hook.HookInput) error {
	input.Values.Set("nodeManager.internal.instancePrefix", instanceprefix.Resolve(input.Values))

	return nil
}
