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

package hooks

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	d8http "github.com/deckhouse/deckhouse/go_lib/dependency/http"
)

// This hook is needed to fill the gaps between Deckhouse restarts and avoid alerts flapping.
// it takes latest metrics from prometheus and duplicate them on Deckhouse startup

func HandleDeprecatedAPIStartup(input *go_hook.HookInput, dc dependency.Container) error {
	input.MetricsCollector.Expire("helm_deprecated_apiversions")

	cl := dc.GetHTTPClient(d8http.WithInsecureSkipVerify())

	// curl -s --connect-timeout 10 --max-time 10 -k -XGET -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" "https://prometheus.d8-monitoring:9090/api/v1/query?query=resource_versions_compatibility"
	promURL := "https://prometheus.d8-monitoring:9090/api/v1/query?query=resource_versions_compatibility"
	req, err := http.NewRequest("GET", promURL, nil)
	if err != nil {
		return err
	}
	err = d8http.SetKubeAuthToken(req)
	if err != nil {
		return err
	}

	res, err := cl.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var response promMetrics
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return err
	}

	for _, metricRecord := range response.Data.Result {
		if len(metricRecord.Value) < 2 {
			input.LogEntry.Warnf("Broken metric value from prometheus: %s. Skipping", metricRecord.Value)
			continue
		}
		value, err := strconv.ParseFloat(metricRecord.Value[1].(string), 64)
		if err != nil {
			input.LogEntry.Warnf("Failed metric convert: %s. Skipping", metricRecord.Value[1])
			continue
		}
		input.MetricsCollector.Set("resource_versions_compatibility", value, map[string]string{
			"helm_release_name":      metricRecord.Metric.HelmReleaseName,
			"helm_release_namespace": metricRecord.Metric.HelmReleaseNamespace,
			"k8s_version":            metricRecord.Metric.K8sVersion,
			"resource_name":          metricRecord.Metric.ResourceName,
			"resource_namespace":     metricRecord.Metric.ResourceNamespace,
			"kind":                   metricRecord.Metric.Kind,
			"api_version":            metricRecord.Metric.APIVersion,
		}, metrics.WithGroup("helm_deprecated_apiversions"))
	}

	return nil
}

type promMetrics struct {
	Data struct {
		Result []struct {
			Metric struct {
				APIVersion           string `json:"api_version"`
				Kind                 string `json:"kind"`
				HelmReleaseName      string `json:"helm_release_name"`
				HelmReleaseNamespace string `json:"helm_release_namespace"`
				K8sVersion           string `json:"k8s_version"`
				ResourceName         string `json:"resource_name"`
				ResourceNamespace    string `json:"resource_namespace"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}
