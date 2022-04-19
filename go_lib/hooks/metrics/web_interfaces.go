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

package metrics

import (
	"path"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
)

func RegisterD8WebInterfaceMetric(name, urlPrefix, urlSuffix string) {
	handlerFunc := func(input *go_hook.HookInput) error {
		publicTemplate := input.Values.Get("global.modules.publicDomainTemplate").String()
		u := strings.ReplaceAll(publicTemplate, "%s", urlPrefix)
		if urlSuffix != "" {
			u = path.Join(u, urlSuffix)
		}
		input.MetricsCollector.Set("deckhouse_web_interfaces", 1, map[string]string{"name": name, "url": u}, metrics.WithGroup("deckhouse_web_interfaces"))

		return nil
	}

	sdk.RegisterFunc(&go_hook.HookConfig{
		Queue:     "deckhouse_web_interfaces",
		OnStartup: &go_hook.OrderedConfig{Order: 10},
	}, handlerFunc)
}
