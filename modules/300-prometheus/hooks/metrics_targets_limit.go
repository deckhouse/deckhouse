/*
Copyright 2022 Flant JSC

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

package hooks

import (
	"encoding/json"
	"fmt"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	d8http "github.com/deckhouse/deckhouse/go_lib/dependency/http"
	"net/http"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/prometheus/metrics_targets_limit",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "targets_limit",
			Crontab: "*/10 * * * *",
		},
	},
}, dependency.WithExternalDependencies(targetsLimitMetricHandler))

const (
	connectBaseURL = "https://prometheus.d8-monitoring:9090"
	apiURL         = connectBaseURL + "/api/v1/targets?state=active&scrapePool="
)

type Target struct {
	LastError  string            `json:"lastError"`
	ScrapePool string            `json:"scrapePool"`
	Labels     map[string]string `json:"labels"`
}

type Labels map[string]string

func makePrometheusRequest(promURL string, dc dependency.Container) ([]Target, error) {
	cl := dc.GetHTTPClient(d8http.WithInsecureSkipVerify())

	req, err := http.NewRequest("GET", promURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create http conection: %v", err)
	}
	err = d8http.SetKubeAuthToken(req)
	if err != nil {
		return nil, err
	}

	res, err := cl.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot http conection: %v", err)
	}
	defer res.Body.Close()

	type data struct {
		Target []Target `json:"activeTargets"`
	}
	type Prom struct {
		Data data `json:"data"`
	}
	var response Prom
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, fmt.Errorf("cannot json parse: %v", err)
	}
	return response.Data.Target, nil
}

func filterTargets(LastError string, list []Target) []Labels {
	var rows []Labels
	for row := range list {
		if list[row].LastError == LastError {
			temp := list[row].Labels
			temp["scrapePool"] = list[row].ScrapePool
			rows = append(rows, temp)
		}
	}
	return rows
}

func targetsLimitMetricHandler(input *go_hook.HookInput, dc dependency.Container) error {
	targets, err := makePrometheusRequest(apiURL, dc)
	if err != nil {
		input.LogEntry.Errorf("cannot makePrometheusRequest: %v", err)
		return nil
	}
	metric := filterTargets("sample limit exceeded", targets)

	input.MetricsCollector.Expire("prometheus_target_limits_hook")
	for row := range metric {
		input.MetricsCollector.Set(
			"d8_prometheus_target_limits_metrics",
			1,
			metric[row],
			metrics.WithGroup("prometheus_target_limits_hook"),
		)
	}

	return nil
}
