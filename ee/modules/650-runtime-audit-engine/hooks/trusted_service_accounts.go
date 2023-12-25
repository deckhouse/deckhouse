/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, addTrustedServiceAccounts)

func addTrustedServiceAccounts(input *go_hook.HookInput) error {
	input.Values.Set("runtimeAuditEngine.internal.trustedServiceAccounts", trustedServiceAccounts)

	return nil
}
