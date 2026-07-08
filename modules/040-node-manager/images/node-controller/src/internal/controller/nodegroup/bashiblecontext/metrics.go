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

// nodeGroupInfo is the node_group_info metric previously emitted by the get_crds
// hook. It carries the resolved CRI type per NodeGroup and feeds the module 340
// UnsupportedContainerRuntimeVersion alert (cri-version.tpl), which joins on the
// name label and filters cri_type != "NotManaged".
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

// setNodeGroupInfo resets the gauge and re-populates it from the assembled blob
// elements, mirroring the get_crds Expire("")+Set-per-NodeGroup behaviour. Each
// element carries the resolved cri.type set by BuildNodeGroupBlob.
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
