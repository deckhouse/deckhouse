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

package hooks

import (
    "github.com/deckhouse/deckhouse/pkg/log"
    "github.com/flant/addon-operator/pkg/module_manager/go_hook"
    "github.com/flant/addon-operator/sdk"
    "github.com/flant/shell-operator/pkg/kube_events_manager/types"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Resource struct {
    APIVersion string
    Kind       string
    Namespace  string
    Name       string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
    OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
    Kubernetes: []go_hook.KubernetesConfig{
        {
            Name:       "deployments",
            ApiVersion: "apps/v1",
            Kind:       "Deployment",
            FilterFunc: applyResourceFilter,
            NameSelector: &types.NameSelector{
                MatchNames: []string{"trickster"},
            },
            NamespaceSelector: &types.NamespaceSelector{
                NameSelector: &types.NameSelector{
                    MatchNames: []string{"d8-monitoring"},
                },
            },
        },
        {
            Name:       "services",
            ApiVersion: "v1",
            Kind:       "Service",
            FilterFunc: applyResourceFilter,
            NameSelector: &types.NameSelector{
                MatchNames: []string{"trickster"},
            },
            NamespaceSelector: &types.NamespaceSelector{
                NameSelector: &types.NameSelector{
                    MatchNames: []string{"d8-monitoring"},
                },
            },
        },
        {
            Name:       "configmaps",
            ApiVersion: "v1",
            Kind:       "ConfigMap",
            FilterFunc: applyResourceFilter,
            NameSelector: &types.NameSelector{
                MatchNames: []string{"trickster-config"},
            },
            NamespaceSelector: &types.NamespaceSelector{
                NameSelector: &types.NameSelector{
                    MatchNames: []string{"d8-monitoring"},
                },
            },
        },
        {
            Name:       "pdbs",
            ApiVersion: "policy/v1",
            Kind:       "PodDisruptionBudget",
            FilterFunc: applyResourceFilter,
            NameSelector: &types.NameSelector{
                MatchNames: []string{"trickster"},
            },
            NamespaceSelector: &types.NamespaceSelector{
                NameSelector: &types.NameSelector{
                    MatchNames: []string{"d8-monitoring"},
                },
            },
        },
        {
            Name:       "podMonitor",
            ApiVersion: "monitoring.coreos.com/v1",
            Kind:       "ServiceMonitor",
            FilterFunc: applyResourceFilter,
            NameSelector: &types.NameSelector{
                MatchNames: []string{"trickster-module"},
            },
            NamespaceSelector: &types.NamespaceSelector{
                NameSelector: &types.NameSelector{
                    MatchNames: []string{"d8-monitoring"},
                },
            },
        },
        {
            Name:       "roles",
            ApiVersion: "rbac.authorization.k8s.io/v1",
            Kind:       "Role",
            FilterFunc: applyResourceFilter,
            NameSelector: &types.NameSelector{
                MatchNames: []string{"access-to-trickster-http", "access-to-trickster-prometheus-metrics"},
            },
            NamespaceSelector: &types.NamespaceSelector{
                NameSelector: &types.NameSelector{
                    MatchNames: []string{"d8-monitoring"},
                },
            },
        },
        {
            Name:       "rolebindings",
            ApiVersion: "rbac.authorization.k8s.io/v1",
            Kind:       "RoleBinding",
            FilterFunc: applyResourceFilter,
            NameSelector: &types.NameSelector{
                MatchNames: []string{"access-to-trickster-http", "access-to-trickster-prometheus-metrics"},
            },
            NamespaceSelector: &types.NamespaceSelector{
                NameSelector: &types.NameSelector{
                    MatchNames: []string{"d8-monitoring"},
                },
            },
        },
        {
            Name:       "serviceaccounts",
            ApiVersion: "v1",
            Kind:       "ServiceAccount",
            FilterFunc: applyResourceFilter,
            NameSelector: &types.NameSelector{
                MatchNames: []string{"trickster"},
            },
            NamespaceSelector: &types.NamespaceSelector{
                NameSelector: &types.NameSelector{
                    MatchNames: []string{"d8-monitoring"},
                },
            },
        },
        {
            Name:       "clusterrolebindings",
            ApiVersion: "rbac.authorization.k8s.io/v1",
            Kind:       "ClusterRoleBinding",
            FilterFunc: applyResourceFilter,
            NameSelector: &types.NameSelector{
                MatchNames: []string{"d8:prometheus:trickster:rbac-proxy"},
            },
        },
    },
}, removeTricksterResources)

func applyResourceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
    meta := obj.Object["metadata"].(map[string]interface{})
    name, _, _ := unstructured.NestedString(meta, "name")
    namespace, _, _ := unstructured.NestedString(meta, "namespace")
    apiVersion, _, _ := unstructured.NestedString(obj.Object, "apiVersion")
    kind := obj.GetKind()

    return &Resource{
        APIVersion: apiVersion,
        Kind:       kind,
        Namespace:  namespace,
        Name:       name,
    }, nil
}

func removeTricksterResources(input *go_hook.HookInput) error {
    for snapshotName, snapshots := range input.Snapshots {
        if len(snapshots) > 0 {
            for _, snap := range snapshots {
                resource := snap.(*Resource)
                log.Debug("Deleting %s: %s/%s from snapshot %s", resource.Kind, resource.Namespace, resource.Name, snapshotName)
                input.PatchCollector.Delete(resource.APIVersion, resource.Kind, resource.Namespace, resource.Name)
            }
        } else {
            log.Debug("No resources found in snapshot %s", snapshotName)
        }
    }
    return nil
}