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

package cloudprovider

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

var providerTypeUnset = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "d8_node_group_provider_type_unset",
		Help: "Set to 1 when the nodes of a NodeGroup run in a cloud the NodeGroup does not declare in spec.providerType",
	},
	[]string{"name", "node_type"},
)

var providerTypeInvalid = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "d8_node_group_provider_type_invalid",
		Help: "Set to 1 when spec.providerType names something other than the cloud provider the nodes of the NodeGroup run in",
	},
	[]string{"name", "node_type", "provider_type"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(providerTypeUnset)
	ctrlmetrics.Registry.MustRegister(providerTypeInvalid)
}

// TrackProviderMetrics publishes the verdict on spec.providerType. The two states are exclusive: a
// declaration that does not hold is reported as invalid, whatever it declares, and only a NodeGroup
// whose declaration is absent is reported as unset.
func TrackProviderMetrics(ng *v1.NodeGroup, provider Provider, resolveErr error) {
	ClearProviderMetrics(ng.Name)
	nodeType := string(ng.Spec.NodeType)

	if resolveErr != nil {
		providerTypeInvalid.WithLabelValues(ng.Name, nodeType, ng.Spec.ProviderType).Set(1)
		return
	}

	if isStatic(ng.Spec.ProviderType) && !provider.IsStatic() {
		providerTypeUnset.WithLabelValues(ng.Name, nodeType).Set(1)
	}
}

// ClearProviderMetrics drops every series of a NodeGroup, so a verdict does not outlive the
// NodeGroup that earned it, nor survive a change of the labels it was published under.
func ClearProviderMetrics(name string) {
	labels := prometheus.Labels{"name": name}
	providerTypeUnset.DeletePartialMatch(labels)
	providerTypeInvalid.DeletePartialMatch(labels)
}
