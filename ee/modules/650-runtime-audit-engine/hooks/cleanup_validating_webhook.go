/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterDeleteHelm: &go_hook.OrderedConfig{Order: 10},
}, removeValidatingWebhook)

func removeValidatingWebhook(_ context.Context, input *go_hook.HookInput) error {
	input.PatchCollector.Delete("admissionregistration.k8s.io/v1", "ValidatingWebhookConfiguration", "", "d8-runtime-audit-engine.deckhouse.io")
	return nil
}
