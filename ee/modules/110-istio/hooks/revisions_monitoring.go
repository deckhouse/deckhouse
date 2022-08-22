/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
)

var (
	revisionsMonitoringMetricsGroup = "revisions"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("revisions-discovery-monitoring"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "namespaces_global_revision",
			ApiVersion: "v1",
			Kind:       "Namespace",
			FilterFunc: applyNamespaceFilter, // from revisions_discovery.go
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"istio-injection": "enabled"},
			},
		},
		{
			Name:       "namespaces_definite_revision",
			ApiVersion: "v1",
			Kind:       "Namespace",
			FilterFunc: applyNamespaceFilter, // from revisions_discovery.go
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "istio.io/rev",
						Operator: "Exists",
					},
				},
			},
		},
		{
			Name:       "istio_pod",
			ApiVersion: "v1",
			Kind:       "Pod",
			FilterFunc: applyIstioPodFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "service.istio.io/canonical-name",
						Operator: "Exists",
					},
					{
						Key:      "sidecar.istio.io/inject",
						Operator: "NotIn",
						Values:   []string{"false"},
					},
				},
			},
		},
	},
}, revisionsMonitoring)

type IstioPodStatus struct {
	Revision string `json:"revision"`
	// ... we aren't interested in the other fields
}

type IstioPodInfo struct {
	Name             string
	Namespace        string
	Revision         string
	SpecificRevision string
}

func getIstioPodSpecificRevision(p *v1.Pod) string {
	var istioStatusJSON string
	var istioPodStatus IstioPodStatus
	var revision string
	var ok bool

	if istioStatusJSON, ok = p.Annotations["sidecar.istio.io/status"]; ok {
		_ = json.Unmarshal([]byte(istioStatusJSON), &istioPodStatus)

		if istioPodStatus.Revision != "" {
			revision = istioPodStatus.Revision
		} else {
			// migration — delete this "else" when v1x10x1 will be retired
			revision = "v1x10x1"
		}
	} else {
		revision = "unknown"
	}

	return revision
}

func getIstioPodDesiredRevision(p *v1.Pod) string {
	var desiredRevision string
	var ok bool
	if desiredRevision, ok = p.Labels["istio.io/rev"]; ok {
		return desiredRevision
	}
	return "unknown"
}

func applyIstioPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pod := &v1.Pod{}
	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, fmt.Errorf("cannot convert pod object to pod: %v", err)
	}

	result := IstioPodInfo{
		Name:             pod.Name,
		Namespace:        pod.Namespace,
		Revision:         getIstioPodSpecificRevision(pod),
		SpecificRevision: getIstioPodDesiredRevision(pod),
	}

	return result, nil
}

func revisionsMonitoring(input *go_hook.HookInput) error {
	if !input.Values.Get("istio.internal.globalRevision").Exists() {
		return nil
	}
	if !input.Values.Get("istio.internal.revisionsToInstall").Exists() {
		return nil
	}

	input.MetricsCollector.Expire(revisionsMonitoringMetricsGroup)

	var globalRevision = input.Values.Get("istio.internal.globalRevision").String()

	var namespaceRevisionMap = map[string]string{}
	for _, ns := range append(input.Snapshots["namespaces_definite_revision"], input.Snapshots["namespaces_global_revision"]...) {
		nsInfo := ns.(NamespaceInfo)
		if nsInfo.Revision == "global" {
			namespaceRevisionMap[nsInfo.Name] = globalRevision
		} else {
			namespaceRevisionMap[nsInfo.Name] = nsInfo.Revision
		}
	}

	for _, pod := range input.Snapshots["istio_pod"] {
		istioPodInfo := pod.(IstioPodInfo)

		desiredRevision := "unknown"
		if desiredRevisionNS, ok := namespaceRevisionMap[istioPodInfo.Namespace]; ok {
			desiredRevision = desiredRevisionNS
		} else {
			// if ns revision set -> override pod revision
			desiredRevision = istioPodInfo.SpecificRevision
		}

		labels := map[string]string{
			"namespace":        istioPodInfo.Namespace,
			"dataplane_pod":    istioPodInfo.Name,
			"desired_revision": desiredRevision,
			"revision":         istioPodInfo.Revision,
		}
		input.MetricsCollector.Set("d8_istio_pod_revision", 1, labels, metrics.WithGroup(revisionsMonitoringMetricsGroup))
	}
	return nil
}
