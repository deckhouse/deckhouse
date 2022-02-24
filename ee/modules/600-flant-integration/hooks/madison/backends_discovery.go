/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package madison

import (
	"fmt"
	"net"
	"net/url"
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
		backendsPath       = "flantIntegration.internal.madison.backends"
		licenseKeyPath     = "flantIntegration.internal.licenseKey"
		httpProxyConfPath  = "global.modules.proxy.httpProxy"
		httpsProxyConfPath = "global.modules.proxy.httpsProxy"
	)

	madisonHost := "https://madison-direct.flant.com:443"

	if input.Values.Get(licenseKeyPath).String() == "" {
		input.Values.Remove(backendsPath)
		return nil
	}

	// check if proxy settings is present
	if p := input.ConfigValues.Get(httpProxyConfPath).String(); p != "" {
		madisonHost = fmt.Sprintf("http://%s", p)
	}
	if p := input.ConfigValues.Get(httpsProxyConfPath).String(); p != "" {
		madisonHost = fmt.Sprintf("https://%s", p)
	}

	u, err := url.Parse(madisonHost)
	if err != nil {
		return err
	}

	addresses, err := net.LookupIP(u.Hostname())
	if err != nil {
		return err
	}

	addressesWithPorts := make([]string, 0, len(addresses))
	for _, v := range addresses {
		addressesWithPorts = append(addressesWithPorts, fmt.Sprintf("%s:%s", v.String(), u.Port()))
	}

	// always keep ip address in the same order to prevent rollouts
	sort.Strings(addressesWithPorts)

	input.Values.Set(backendsPath, addressesWithPorts)
	return nil
}
