/*
Copyright 2025 Flant JSC
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

package watcher

const (
	labelThresholdPrefix   = "threshold.extended-monitoring.deckhouse.io/"
	namespacesEnabledLabel = "extended-monitoring.deckhouse.io/enabled"
)

var nodeThresholdMap = map[string]float64{
	"disk-bytes-warning":             70,
	"disk-bytes-critical":            80,
	"disk-inodes-warning":            90,
	"disk-inodes-critical":           95,
	"load-average-per-core-warning":  3,
	"load-average-per-core-critical": 10,
}

var podThresholdMap = map[string]float64{
	"disk-bytes-warning":   85,
	"disk-bytes-critical":  95,
	"disk-inodes-warning":  85,
	"disk-inodes-critical": 90,
}

var daemonSetThresholdMap = map[string]float64{
	"replicas-not-ready": 0,
}

var statefulSetThresholdMap = map[string]float64{
	"replicas-not-ready": 0,
}

var deploymentThresholdMap = map[string]float64{
	"replicas-not-ready": 0,
}

var ingressThresholdMap = map[string]float64{
	"5xx-warning":  10,
	"5xx-critical": 20,
}
