// Copyright 2021 Flant CJSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deckhouse

import "github.com/flant/shell-operator/pkg/metric_storage"

func RegisterDeckhouseMetrics(metricStorage *metric_storage.MetricStorage) {
	// Metrics for retrieving image digest from Pod status.
	metricStorage.RegisterCounter("deckhouse_kube_image_digest_check_total", map[string]string{})
	metricStorage.CounterAdd("deckhouse_kube_image_digest_check_total", 0.0, map[string]string{})
	metricStorage.RegisterGauge("deckhouse_kube_image_digest_check_success", map[string]string{})
	metricStorage.GaugeSet("deckhouse_kube_image_digest_check_success", 0.0, map[string]string{})

	// Metrics for checking image in Docker registry.
	// This checking starts when deckhouse_kube_image_digest_check_success become 1.
	metricStorage.RegisterCounter("deckhouse_registry_check_total", map[string]string{})
	metricStorage.CounterAdd("deckhouse_registry_check_total", 0.0, map[string]string{})
	metricStorage.RegisterCounter("deckhouse_registry_check_errors_total", map[string]string{})
	metricStorage.CounterAdd("deckhouse_registry_check_errors_total", 0.0, map[string]string{})
}
