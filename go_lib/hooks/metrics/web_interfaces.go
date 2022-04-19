package metrics

import (
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
)

func RegisterD8WebInterfaceMetric(name string) {
	handlerFunc := func(input *go_hook.HookInput) error {
		publicTemplate := input.Values.Get("global.modules.publicDomainTemplate").String()
		u := strings.ReplaceAll(publicTemplate, "%s", name)
		input.MetricsCollector.Set("deckhouse_web_interfaces", 1, map[string]string{"name": name, "url": u}, metrics.WithGroup("deckhouse_web_interfaces"))

		return nil
	}

	sdk.RegisterFunc(&go_hook.HookConfig{
		Queue:     "deckhouse_web_interfaces",
		OnStartup: &go_hook.OrderedConfig{Order: 10},
	}, handlerFunc)
}
