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

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:     "deckhouse_web_interfaces",
	OnStartup: &go_hook.OrderedConfig{Order: 5},
}, cleanMetrics)

func cleanMetrics(input *go_hook.HookInput) error {
	fmt.Println("CLEANMET")
	input.MetricsCollector.Expire("deckhouse_web_interfaces")

	return nil
}
