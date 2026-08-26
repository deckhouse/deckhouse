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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	reservedPublicHostsValuePath = "deckhouse.internal.reservedPublicHosts"
	reservedPublicHostsQueue     = "/modules/deckhouse/reserved-public-hosts"
)

type reservedPublicHostsSnapshot struct {
	Recorded bool     `json:"recorded"`
	Hosts    []string `json:"hosts"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        reservedPublicHostsQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(snapshotReservedPublicHosts))

func snapshotReservedPublicHosts(_ context.Context, input *go_hook.HookInput, _ dependency.Container) error {
	// Admission is not rendered in this patch. Do not snapshot the cluster: a previous
	// converge may have left grandfatheredHosts, and listing Ingresses is the cost of
	// a reservation that is not in force.
	input.Values.Set(reservedPublicHostsValuePath, reservedPublicHostsSnapshot{Hosts: []string{}})
	return nil
}
