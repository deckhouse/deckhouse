/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterDeleteHelm: &go_hook.OrderedConfig{Order: 10},
}, removeValidatingWebhook)

func removeValidatingWebhook(input *go_hook.HookInput) error {
	input.PatchCollector.Delete("admissionregistration.k8s.io/v1", "ValidatingWebhookConfiguration", "", "d8-runtime-audit-engine.deckhouse.io", object_patch.InForeground())
	return nil
}
