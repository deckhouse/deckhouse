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
	"fmt"
	"strings"

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

// TrackValidateNodeGroupMetrics publishes what is wrong with a NodeGroup's spec.providerType. The
// two states are exclusive: a field that names the wrong provider is reported as invalid, and only
// a field that names nothing at all is reported as unset.
func TrackValidateNodeGroupMetrics(ng *v1.NodeGroup, provider Provider) {
	ClearValidateNodeGroupMetrics(ng.Name)
	nodeType := string(ng.Spec.NodeType)

	if ValidateNodeGroup(ng, provider) != nil {
		providerTypeInvalid.WithLabelValues(ng.Name, nodeType, ng.Spec.ProviderType).Set(1)
		return
	}

	// Declares nothing, and there is a cloud to declare. 'None' is a declaration, and a wrong one
	// over a cloud is already reported above.
	if ng.Spec.ProviderType == "" && !provider.IsStatic() {
		providerTypeUnset.WithLabelValues(ng.Name, nodeType).Set(1)
	}
}

// ClearValidateNodeGroupMetrics drops every series of a NodeGroup, so a verdict does not outlive the
// NodeGroup that earned it, nor survive a change of the labels it was published under.
func ClearValidateNodeGroupMetrics(name string) {
	labels := prometheus.Labels{"name": name}
	providerTypeUnset.DeletePartialMatch(labels)
	providerTypeInvalid.DeletePartialMatch(labels)
}

// ValidateNodeGroup reports why a NodeGroup's spec.providerType disagrees with the provider
// it resolved to, or nil when the two agree. Nothing is published.
//
// The field declares an answer, it does not pick one: leaving it empty is always correct, and
// naming anything other than the resolved provider is a statement about the NodeGroup that a
// retry cannot fix.
func ValidateNodeGroup(ng *v1.NodeGroup, provider Provider) error {
	ngPType := ng.Spec.ProviderType

	// An empty field declares nothing, and declaring nothing is always correct.
	if ngPType == "" {
		return nil
	}

	switch {
	case isStatic(ngPType):
		if provider.IsStatic() {
			return nil
		}
		return fmt.Errorf(
			"Invalid providerType '%s'. The nodes of this group run in the '%s' cloud. "+
				"Please remove the field or set it to '%s'.",
			ngPType, provider.Type, provider.Type)

	case provider.IsStatic():
		return fmt.Errorf(
			"Invalid providerType '%s'. The nodes of this group run in no cloud. "+
				"Please remove the field or set it to 'None'.",
			ngPType)

	case !strings.EqualFold(ngPType, provider.Type):
		return fmt.Errorf(
			"Invalid providerType '%s'. Expected '%s'. Please update the NodeGroup to name the "+
				"cloud provider its nodes run in.",
			ngPType, provider.Type)
	}
	return nil
}
