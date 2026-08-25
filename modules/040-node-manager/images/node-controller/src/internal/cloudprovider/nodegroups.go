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
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

var ErrProviderTypeInvalid = errors.New("Provider type invalid")

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

// TrackNodeGroupMetrics publishes what is wrong with a NodeGroup's spec.providerType. The
// two states are exclusive: a field that names the wrong provider is reported as invalid, and only
// a field that names nothing at all is reported as unset.
func TrackNodeGroupMetrics(ng *v1.NodeGroup, provider Provider) {
	ClearNodeGroupMetrics(ng.Name)
	nodeType := string(ng.Spec.NodeType)

	if ValidateNodeGroupPType(ng, provider) != nil {
		providerTypeInvalid.WithLabelValues(ng.Name, nodeType, ng.Spec.ProviderType).Set(1)
	}

	if ValidateNodeGroupPTypeUnset(ng, provider) {
		providerTypeUnset.WithLabelValues(ng.Name, nodeType).Set(1)
	}
}

// ClearNodeGroupMetrics drops every series of a NodeGroup, so a verdict does not outlive the
// NodeGroup that earned it, nor survive a change of the labels it was published under.
func ClearNodeGroupMetrics(name string) {
	labels := prometheus.Labels{"name": name}
	providerTypeUnset.DeletePartialMatch(labels)
	providerTypeInvalid.DeletePartialMatch(labels)
}

// ValidateNodeGroupPType reports why a NodeGroup's spec.providerType contradicts the provider its
// nodes run in, or nil when it does not.
//
// An empty field is never a defect: the field declares an answer, it does not pick one. Whether a
// group that declares nothing still has a provider left to name is ValidateNodeGroupPTypeUnset.
func ValidateNodeGroupPType(ng *v1.NodeGroup, provider Provider) error {
	declared := ng.Spec.ProviderType
	if declared == "" {
		return nil
	}

	switch ng.Spec.NodeType {
	// Static: the nodes run outside every cloud, whatever the cluster itself runs in, so the field
	// may only say so.
	case v1.NodeTypeStatic:
		if !isStatic(declared) {
			return errRunsInNoCloud(declared)
		}
		return nil

	// CloudEphemeral and CloudPermanent: they always run in the
	// cluster's own cloud and the field has to name it exactly.
	case v1.NodeTypeCloudEphemeral, v1.NodeTypeCloudPermanent:
		if declared != provider.Type {
			return errRunsInAnotherCloud(declared, provider.Type)
		}
		return nil

	// CloudStatic: did not order these nodes, so they follow the cluster — its cloud
	// when it has one, no cloud when it does not.
	case v1.NodeTypeCloudStatic:
		if provider.IsStatic() {
			if !isStatic(declared) {
				return errRunsInNoCloud(declared)
			}
			return nil
		}

		if declared != provider.Type {
			return errRunsInAnotherCloud(declared, provider.Type)
		}
		return nil
	}

	return nil
}

// ValidateNodeGroupPTypeUnset reports a NodeGroup that has a cloud provider to name and names none.
// It is not a defect: the field is being rolled out, and the gauge behind this asks the owner to
// fill it in before it starts to matter.
func ValidateNodeGroupPTypeUnset(ng *v1.NodeGroup, provider Provider) bool {
	// A group that names something is not unset, whatever it names: whether the name holds is
	// ValidateNodeGroupPType.
	if ng.Spec.ProviderType != "" {
		return false
	}

	switch ng.Spec.NodeType {
	// Static: nothing to name, so nothing to ask its owner for.
	case v1.NodeTypeStatic:
		return false

	// CloudEphemeral and CloudPermanent: these always run in a cloud, so the field is always due.
	case v1.NodeTypeCloudEphemeral, v1.NodeTypeCloudPermanent:
		return true

	// CloudStatic and anything added later: due only where the cluster has a cloud to name.
	default:
		return !provider.IsStatic()
	}
}

func errRunsInNoCloud(declared string) error {
	return fmt.Errorf(
		"%w: %q. The nodes of this group run in no cloud. Please remove the field or set it to 'None'.",
		ErrProviderTypeInvalid, declared)
}

func errRunsInAnotherCloud(declared, resolved string) error {
	return fmt.Errorf(
		"%w: %q. Expected %q. Please update the NodeGroup to name the cloud provider its nodes run in.",
		ErrProviderTypeInvalid, declared, resolved)
}
