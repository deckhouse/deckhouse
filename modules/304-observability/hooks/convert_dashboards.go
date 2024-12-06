/*
Copyright 2021 Flant JSC

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
	"encoding/json"
	"errors"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ClusterDashboardKind      = "ClusterObservabilityDashboard"
	PropagatedDashboardKind   = "ClusterObservabilityPropagatedDashboard"
	LegacyDashboardDefinition = "GrafanaDashboardDefinition"
	JsonIndentCharacter       = " "
)

var propagatedDashboards = map[string]bool{
	"d8-admission-policy-engine-security-admission-policy-engine": true,
	"d8-applications-elasticsearch":                               true,
	"d8-applications-etcd3":                                       true,
	"d8-applications-loki":                                        true,
	"d8-applications-memcached":                                   true,
	"d8-applications-mongodb":                                     true,
	"d8-applications-nats":                                        true,
	"d8-applications-nats-legacy":                                 true,
	"d8-applications-pgbouncer":                                   true,
	"d8-applications-php-fpm":                                     true,
	"d8-applications-prometheus":                                  true,
	"d8-applications-rabbitmq":                                    true,
	"d8-applications-rabbitmq-legacy":                             true,
	"d8-applications-redis":                                       true,
	"d8-applications-sidekiq":                                     true,
	"d8-applications-uwsgi":                                       true,
	"d8-monitoring-kubernetes-main-controller":                    true,
	"d8-monitoring-kubernetes-main-namespace-namespace":           true,
	"d8-monitoring-kubernetes-main-namespace-namespaces":          true,
	"d8-monitoring-kubernetes-main-pod":                           true,
	"d8-ingress-nginx-ingress-nginx-namespace-namespace-detail":   true,
	"d8-ingress-nginx-ingress-nginx-namespace-namespaces":         true,
	"d8-ingress-nginx-ingress-nginx-vhost-vhost-detail":           true,
	"d8-ingress-nginx-ingress-nginx-vhost-vhosts":                 true,
	"d8-loki-applications-loki-search":                            true,
}

type LegacyDashboard struct {
	Name       string
	Folder     string
	Definition string
}

func (d *LegacyDashboard) PrefixUid(prefix string) error {
	var dashboard map[string]interface{}

	if err := json.Unmarshal([]byte(d.Definition), &dashboard); err != nil {
		return err
	}

	uid, ok := dashboard["uid"]
	if !ok {
		return errors.New("dashboard definition contains no uid field")
	}

	dashboardUID, ok := uid.(string)
	if !ok {
		return errors.New("dashboard definition uid field is not a string")
	}

	dashboard["uid"] = prefix + dashboardUID

	ret, err := json.MarshalIndent(dashboard, "", strings.Repeat(JsonIndentCharacter, 4))
	if err != nil {
		return err
	}

	d.Definition = string(ret)

	return nil
}

func filterLegacyDashboard(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cr := &LegacyDashboard{}
	cr.Name = obj.GetName()
	spec, ok, err := unstructured.NestedStringMap(obj.Object, "spec")
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, errors.New("no spec.definition field")
	}

	cr.Definition = spec["definition"]
	cr.Folder = spec["folder"]
	return cr, nil
}

func filterDashboardName(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/observability/convert_dashboards",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cluster_observability_dashboards",
			ApiVersion: "observability.deckhouse.io/v1alpha1",
			Kind:       ClusterDashboardKind,
			FilterFunc: filterDashboardName,
		},
		{
			Name:       "propagated_observability_dashboards",
			ApiVersion: "observability.deckhouse.io/v1alpha1",
			Kind:       PropagatedDashboardKind,
			FilterFunc: filterDashboardName,
		},
		{
			Name:       "legacy_dashboards",
			ApiVersion: "deckhouse.io/v1",
			Kind:       LegacyDashboardDefinition,
			FilterFunc: filterLegacyDashboard,
		},
	},
}, convertDashboards)

func convertDashboards(input *go_hook.HookInput) error {
	dashboards := make(map[string]bool)

	legacyDashboardsSnap := input.Snapshots["legacy_dashboards"]

	for _, snap := range legacyDashboardsSnap {
		dash := snap.(*LegacyDashboard)
		kind := dashboardKindByName(dash.Name)
		prefix := dashboardPrefixByKind(kind)

		if err := dash.PrefixUid(prefix); err != nil {
			log.Error("Failed to prefix uid for dashboard", dash.Name, err)
			continue
		}

		input.PatchCollector.Create(
			dashboardManifest(dash.Name, dash.Folder, kind, dash.Definition),
			object_patch.UpdateIfExists(),
		)

		dashboards[dash.Name] = true
	}

	clusterObservabilityDashboardsSnap := input.Snapshots["cluster_observability_dashboards"]
	propagatedObservabilityDashboardsSnap := input.Snapshots["propagated_observability_dashboards"]

	// delete ClusterObservabilityDashboard and PropagatedObservabilityDashboard if no corresponding GrafanaDashboardDefinition found
	for _, sn := range clusterObservabilityDashboardsSnap {
		resourceName := sn.(string)
		if _, ok := dashboards[resourceName]; !ok {
			input.PatchCollector.Delete("observability.deckhouse.io/v1alpha1", ClusterDashboardKind, "", resourceName)
		}
	}

	for _, sn := range propagatedObservabilityDashboardsSnap {
		resourceName := sn.(string)
		if _, ok := dashboards[resourceName]; !ok {
			input.PatchCollector.Delete("observability.deckhouse.io/v1alpha1", PropagatedDashboardKind, "", resourceName)
		}
	}

	return nil
}

func dashboardKindByName(name string) string {
	if _, ok := propagatedDashboards[name]; ok {
		return PropagatedDashboardKind
	}

	return ClusterDashboardKind
}

func dashboardPrefixByKind(kind string) string {
	if kind == PropagatedDashboardKind {
		return "propagated_"
	}

	return "cluster_"
}

func dashboardManifest(name, category, kind, definition string) *unstructured.Unstructured {
	un := unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "observability.deckhouse.io/v1alpha1",
		"kind":       kind,
		"metadata": map[string]interface{}{
			"name": name,
			"annotations": map[string]interface{}{
				"observability.deckhouse.io/category": category,
			},
			"labels": map[string]interface{}{
				"module":   "observability",
				"heritage": "deckhouse",
			},
		},
		"spec": map[string]interface{}{
			"definition": definition,
		},
	}}

	return &un
}
