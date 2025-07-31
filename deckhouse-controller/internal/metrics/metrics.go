// Copyright 2025 Flant JSC
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

package metrics

import "github.com/flant/shell-operator/pkg/metric"

func RegisterMetrics(metricStorage metric.Storage) {
	registerModuleMetrics(metricStorage)
}

const (
	MigratedModuleNotFoundMetricName = "d8_migrated_module_not_found"
	MigratedModuleNotFoundGroup      = "migrated_module_not_found"
)

func registerModuleMetrics(metricStorage metric.Storage) {
	// metricStorage.Grouped().GaugeSet(
	// 	MigratedModuleNotFoundGroup,
	// 	MigratedModuleNotFoundMetricName,
	// 	0,
	// 	map[string]string{
	// 		"module_name": "",
	// 	})
}
