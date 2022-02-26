/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package madison

import (
	"bytes"
	"net"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/flant-integration/madison_backends_discovery",
	AllowFailure: true,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "madison_backends_discovery",
			Crontab: "*/10 * * * *",
		},
	},
}, backendsHandler)

func backendsHandler(input *go_hook.HookInput) error {
	const (
		backendsPath   = "flantIntegration.internal.madison.backends"
		licenseKeyPath = "flantIntegration.internal.licenseKey"
	)

	if input.Values.Get(licenseKeyPath).String() == "" {
		input.Values.Remove(backendsPath)
		return nil
	}

	addresses, err := net.LookupIP("madison-direct.flant.com")
	if err != nil {
		return err
	}

	// always keep ip address in the same order to prevent rollouts
	sort.Slice(addresses, func(i, j int) bool {
		return bytes.Compare(addresses[i], addresses[j]) < 0
	})

	input.Values.Set(backendsPath, addresses)
	return nil
}
