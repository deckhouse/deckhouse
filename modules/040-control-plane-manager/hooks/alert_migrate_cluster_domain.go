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

// DefaultClusterDomain is used when neither document sets the domain.
const DefaultClusterDomain = "cluster.local"

// Reads the Secret directly: global.clusterConfiguration.clusterDomain carries the resolved value and
// is never empty, so it cannot tell whether the deprecated field is still there.
const (
	obsoleteClusterDomainMetricGroup = "D8ObsoleteClusterDomainInClusterConfiguration"
	obsoleteClusterDomainMetricName  = "d8_obsolete_cluster_domain_in_cluster_configuration"

	obsoleteClusterDomainSnapshot = "clusterConfigurationClusterDomain"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/control-plane-manager/alerting",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              obsoleteClusterDomainSnapshot,
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}}},
			NameSelector:      &types.NameSelector{MatchNames: []string{"d8-cluster-configuration"}},
			FilterFunc:        filterClusterConfigurationClusterDomain,
		},
	},
}, checkClusterDomainMigration)

type clusterConfigurationClusterDomain struct {
	ClusterDomain string `json:"clusterDomain"`
}

func filterClusterConfigurationClusterDomain(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	fields := clusterConfigurationClusterDomain{}
	// Best-effort: a malformed document is the discovery hook's problem to report.
	_ = yaml.Unmarshal(secret.Data["cluster-configuration.yaml"], &fields)
	return fields, nil
}

func checkClusterDomainMigration(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(obsoleteClusterDomainMetricGroup)

	snap, err := sdkobjectpatch.UnmarshalToStruct[clusterConfigurationClusterDomain](input.Snapshots, obsoleteClusterDomainSnapshot)
	if err != nil {
		return fmt.Errorf("failed to unmarshal %s snapshot: %w", obsoleteClusterDomainSnapshot, err)
	}
	if len(snap) == 0 || snap[0].ClusterDomain == "" {
		return nil
	}

	input.MetricsCollector.Set(
		obsoleteClusterDomainMetricName, 1,
		map[string]string{},
		metrics.WithGroup(obsoleteClusterDomainMetricGroup),
	)

	return nil
}
