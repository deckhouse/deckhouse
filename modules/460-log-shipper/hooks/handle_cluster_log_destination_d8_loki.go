/*
Copyright 2024 Flant JSC

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
	"net/url"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/pkg/errors"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

const (
	lokiAuthorizationRequiredGroup      = "loki_authorization_required"
	lokiAuthorizationRequiredMetricName = "d8_log_shipper_cluster_log_destination_d8_loki_authorization_required"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/log-shipper/cluster_log_destination_d8_loki",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cluster_log_destination",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ClusterLogDestination",
			FilterFunc: filterClusterLogDestination,
		},
		{
			Name:       "loki_endpoint",
			ApiVersion: "v1",
			Kind:       "Endpoints",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{
					"d8-monitoring",
				}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"loki",
			}},
			FilterFunc: filterLokiEndpoints,
		},
	},
}, handleClusterLogDestinationD8Loki)

func handleClusterLogDestinationD8Loki(input *go_hook.HookInput) error {
	destinationSnapshots := input.Snapshots["cluster_log_destination"]
	lokiEndpointSnap := input.Snapshots["loki_endpoint"]

	var lokiEndpoint endpoint

	if len(lokiEndpointSnap) > 0 {
		lokiEndpoint = lokiEndpointSnap[0].(endpoint)
	}

	clusterDomain := input.Values.Get("global.discovery.clusterDomain").String()

	input.MetricsCollector.Expire(lokiAuthorizationRequiredGroup)

	for _, destinationSnapshot := range destinationSnapshots {
		destination := destinationSnapshot.(v1alpha1.ClusterLogDestination)

		if destination.Name == "d8-loki" {
			continue
		}

		if destination.Spec.Type != v1alpha1.DestLoki {
			continue
		}

		endpointURL, err := url.Parse(destination.Spec.Loki.Endpoint)
		if err != nil {
			return errors.Wrapf(err, "failed to parse loki endpoint '%s'", destination.Spec.Loki.Endpoint)
		}

		if !matchLokiEndpoint(endpointURL.Host, clusterDomain, lokiEndpoint) {
			continue
		}

		if endpointURL.Scheme == "https" &&
			destination.Spec.Loki.Auth.Strategy == "Bearer" &&
			destination.Spec.Loki.Auth.Token != "" {
			continue
		}

		input.MetricsCollector.Set(lokiAuthorizationRequiredMetricName, 1, map[string]string{"resource_name": destination.Name}, metrics.WithGroup(lokiAuthorizationRequiredGroup))
	}

	return nil
}
