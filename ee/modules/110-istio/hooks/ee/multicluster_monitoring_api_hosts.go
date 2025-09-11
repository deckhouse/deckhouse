/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/http"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
	"github.com/deckhouse/deckhouse/pkg/log"
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

func setAPIHostMetric(mc sdkpkg.MetricsCollector, name, apiHost string, isError float64) {
	labels := map[string]string{
		"multicluster_name": name,
		"api_host":          apiHost,
	}
	mc.Set(multiclusterMonitoringMetricName, isError, labels, metrics.WithGroup(multiclusterMonitoringMetricsGroup))
}

func monitoringAPIHosts(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	if !input.Values.Get("istio.multicluster.enabled").Bool() {
		return nil
	}

	input.MetricsCollector.Expire(multiclusterMonitoringMetricsGroup)

	multiclusters := input.Values.Get("istio.internal.multiclusters").Array()
	for _, m := range multiclusters {
		name := m.Get("name").String()
		apiHost := m.Get("apiHost").String()
		apiJWT := m.Get("apiJWT").String()
		apiSkipVerify := m.Get("insecureSkipVerify").Bool()
		apiAdditionalCA := m.Get("ca").String()

		var options []http.Option
		if apiSkipVerify {
			options = append(options, http.WithInsecureSkipVerify())
		}
		if apiAdditionalCA != "" {
			caCerts := [][]byte{[]byte(apiAdditionalCA)}
			options = append(options, http.WithAdditionalCACerts(caCerts))
		}

		bodyBytes, statusCode, err := lib.HTTPGet(dc.GetHTTPClient(options...), fmt.Sprintf("https://%s/api", apiHost), apiJWT)
		if err != nil {
			input.Logger.Warn("cannot fetch api host for IstioMulticluster", slog.String("api_host", apiHost), slog.String("name", name), log.Err(err))
			setAPIHostMetric(input.MetricsCollector, name, apiHost, 1)
			continue
		}
		if statusCode != 200 {
			input.Logger.Warn("cannot fetch api host for IstioMulticluster", slog.String("api_host", apiHost), slog.String("name", name), slog.Int("http_code", statusCode))
			setAPIHostMetric(input.MetricsCollector, name, apiHost, 1)
			continue
		}

		var response apiResponse
		err = json.Unmarshal(bodyBytes, &response)
		if err != nil {
			input.Logger.Warn("cannot unmarshal api host response for IstioMulticluster", slog.String("api_host", apiHost), slog.String("name", name), log.Err(err))
			setAPIHostMetric(input.MetricsCollector, name, apiHost, 1)
			continue
		}

		if response.Kind != "APIVersions" {
			input.Logger.Warn("got wrong response format from api host for IstioMulticluster", slog.String("api_host", apiHost), slog.String("name", name))
			setAPIHostMetric(input.MetricsCollector, name, apiHost, 1)
			continue
		}

		setAPIHostMetric(input.MetricsCollector, name, apiHost, 0)
	}

	return nil
}
