/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:     internal.Queue("main"),
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, labelD8System)

// Migration — delete after the first execution
func labelD8System(input *go_hook.HookInput) error {
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				"istio.deckhouse.io/discovery": "disabled",
			},
		},
	}
	input.PatchCollector.MergePatch(patch, "v1", "Namespace", "", "d8-system")

	return nil
}
