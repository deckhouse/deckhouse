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

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

/*
Description:
	Alerts while any of the three deprecated ClusterConfiguration network fields
	(podSubnetCIDR, serviceSubnetCIDR, podSubnetNodeCIDRPrefix) is still present,
	asking to move them into ModuleConfig control-plane-manager.

	Reads the d8-cluster-configuration Secret directly, not
	global.clusterConfiguration.*: the global discovery hook (cluster_configuration.go)
	substitutes the *resolved* value (ModuleConfig, else the deprecated field) into that
	key so every template keeps working unedited, which means the key stays non-empty
	forever once ModuleConfig sets it - even after the deprecated field is removed from
	ClusterConfiguration. A direct Kubernetes binding on the Secret also makes this hook
	re-run exactly when the Secret changes, rather than depending on some other hook
	touching global values to trigger a rerun.
*/

const (
	obsoleteNetworkFieldsMetricGroup = "D8ObsoleteNetworkFieldsInClusterConfiguration"
	obsoleteNetworkFieldsMetricName  = "d8_obsolete_network_fields_in_cluster_configuration"

	obsoleteNetworkFieldsSnapshot = "clusterConfigurationNetworkFields"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/control-plane-manager/alerting",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              obsoleteNetworkFieldsSnapshot,
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}}},
			NameSelector:      &types.NameSelector{MatchNames: []string{"d8-cluster-configuration"}},
			FilterFunc:        filterClusterConfigurationNetworkFields,
		},
	},
}, checkNetworkFieldsMigration)

type clusterConfigurationNetworkFields struct {
	PodSubnetCIDR           string `json:"podSubnetCIDR"`
	ServiceSubnetCIDR       string `json:"serviceSubnetCIDR"`
	PodSubnetNodeCIDRPrefix string `json:"podSubnetNodeCIDRPrefix"`
}

func filterClusterConfigurationNetworkFields(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	fields := clusterConfigurationNetworkFields{}
	// Best-effort: a malformed document is the discovery hook's problem to report; this hook only
	// asks whether the three deprecated fields are still present.
	_ = yaml.Unmarshal(secret.Data["cluster-configuration.yaml"], &fields)
	return fields, nil
}

func checkNetworkFieldsMigration(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(obsoleteNetworkFieldsMetricGroup)

	snap, err := sdkobjectpatch.UnmarshalToStruct[clusterConfigurationNetworkFields](input.Snapshots, obsoleteNetworkFieldsSnapshot)
	if err != nil {
		return fmt.Errorf("failed to unmarshal %s snapshot: %w", obsoleteNetworkFieldsSnapshot, err)
	}
	if len(snap) == 0 {
		return nil
	}

	fields := snap[0]
	if fields.PodSubnetCIDR == "" && fields.ServiceSubnetCIDR == "" && fields.PodSubnetNodeCIDRPrefix == "" {
		return nil
	}

	input.MetricsCollector.Set(
		obsoleteNetworkFieldsMetricName, 1,
		map[string]string{},
		metrics.WithGroup(obsoleteNetworkFieldsMetricGroup),
	)

	return nil
}
