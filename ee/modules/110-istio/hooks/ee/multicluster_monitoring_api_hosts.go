/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

var (
	multiclusterMonitoringMetricsGroup = "multicluster_check_api_host"
	multiclusterMonitoringMetricName   = "d8_istio_multicluster_api_host_check_error_count"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: lib.Queue("monitoring"),
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "* * * * *"},
	},
}, dependency.WithExternalDependencies(monitoringAPIHosts))

type apiResponse struct {
	Kind     string   `json:"kind"`
	Versions []string `json:"versions"`
}

func setAPIHostMetric(mc go_hook.MetricsCollector, name, apiHost string, isError float64) {
	labels := map[string]string{
		"multicluster_name": name,
		"api_host":          apiHost,
	}
	mc.Set(multiclusterMonitoringMetricName, isError, labels, metrics.WithGroup(multiclusterMonitoringMetricsGroup))
}

func monitoringAPIHosts(input *go_hook.HookInput, dc dependency.Container) error {
	if !input.Values.Get("istio.multicluster.enabled").Bool() {
		return nil
	}

	input.MetricsCollector.Expire(multiclusterMonitoringMetricsGroup)

	multiclusters := input.Values.Get("istio.internal.multiclusters").Array()
	for _, m := range multiclusters {
		name := m.Get("name").String()
		apiHost := m.Get("apiHost").String()
		apiJWT := m.Get("apiJWT").String()

		bodyBytes, statusCode, err := lib.HTTPGet(dc.GetHTTPClient(), fmt.Sprintf("https://%s/api", apiHost), apiJWT)
		if err != nil {
			input.LogEntry.Warnf("cannot fetch api host %s for IstioMulticluster %s, error: %s", apiHost, name, err.Error())
			setAPIHostMetric(input.MetricsCollector, name, apiHost, 1)
			continue
		}
		if statusCode != http.StatusOK {
			input.LogEntry.Warnf("cannot fetch api host %s for IstioMulticluster %s (HTTP code %d)", apiHost, name, statusCode)
			setAPIHostMetric(input.MetricsCollector, name, apiHost, 1)
			continue
		}

		var response apiResponse
		err = json.Unmarshal(bodyBytes, &response)
		if err != nil {
			input.LogEntry.Warnf("cannot unmarshal api host %s response for IstioMulticluster %s, error: %s", apiHost, name, err.Error())
			setAPIHostMetric(input.MetricsCollector, name, apiHost, 1)
			continue
		}

		if response.Kind != "APIVersions" {
			input.LogEntry.Warnf("got wrong response format from api host %s for IstioMulticluster %s", apiHost, name)
			setAPIHostMetric(input.MetricsCollector, name, apiHost, 1)
			continue
		}

		setAPIHostMetric(input.MetricsCollector, name, apiHost, 0)
	}

	return nil
}
