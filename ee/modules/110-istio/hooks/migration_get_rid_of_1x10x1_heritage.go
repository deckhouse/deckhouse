/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// TODO: Remove me after the hook being deployed to all clusters!

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, removeKalaiCRD)

func removeKalaiCRD(input *go_hook.HookInput) error {
	input.PatchCollector.Delete("apiextensions.k8s.io/v1", "CustomResourceDefinition", "", "monitoringdashboards.monitoring.kiali.io", object_patch.InForeground())
	input.PatchCollector.Delete("admissionregistration.k8s.io/v1", "ValidatingWebhookConfiguration", "", "istiod-d8-istio", object_patch.InForeground())
	return nil
}
