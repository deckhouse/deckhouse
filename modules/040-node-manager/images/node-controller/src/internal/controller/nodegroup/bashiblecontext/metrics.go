/*
Copyright 2026 Flant JSC

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

package bashiblecontext

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var nodeGroupInfo = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "node_group_info",
		Help: "Info about node groups, labelled by the resolved CRI type.",
	},
	[]string{"name", "cri_type"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(nodeGroupInfo)
}

func setNodeGroupInfo(elements []map[string]interface{}) {
	nodeGroupInfo.Reset()
	for _, element := range elements {
		name, _ := element["name"].(string)
		if name == "" {
			continue
		}
		var criType string
		if cri, ok := element["cri"].(map[string]interface{}); ok {
			criType, _ = cri["type"].(string)
		}
		nodeGroupInfo.WithLabelValues(name, criType).Set(1)
	}
}
