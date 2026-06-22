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

package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	valuesPath   = "registry.internal.cache"
	upstreamPath = "registry.upstream"
	cachePath    = "registry.cache"
	queue        = "/modules/registry/cache"
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
		Queue:        queue,
	},
	handle,
)

func handle(_ context.Context, input *go_hook.HookInput) error {
	values := helpers.NewValuesAccessor[CacheValues](input, valuesPath)

	cacheCfg, err := helpers.GetValue[CacheConfig](input, cachePath)
	if err != nil {
		if errors.Is(err, helpers.ErrNoValue) {
			values.Clear()
			return nil
		}
		return fmt.Errorf("read registry.cache: %w", err)
	}

	upstream, err := helpers.GetValue[UpstreamConfig](input, upstreamPath)
	var upstreamPtr *UpstreamConfig
	if err == nil {
		upstreamPtr = &upstream
	} else if !errors.Is(err, helpers.ErrNoValue) {
		return fmt.Errorf("read registry.upstream: %w", err)
	}

	resolved, err := Resolve(upstreamPtr, cacheCfg)
	if err != nil {
		return fmt.Errorf("resolve cache values: %w", err)
	}

	values.Set(resolved)
	return nil
}
