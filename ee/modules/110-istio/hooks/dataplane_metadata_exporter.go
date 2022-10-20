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
	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal/istio_versions"
)

const (
	istioRevsionAbsent           = "absent"
	istioVersionAbsent           = "absent"
	istioVersionUnknown          = "unknown"
	istioPodMetadataMetricName   = "d8_istio_dataplane_metadata"
	metadataExporterMetricsGroup = "metadata"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("dataplane-metadata"),
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
						Key:      "job-name",
						Operator: "DoesNotExist",
					},
					{
						Key:      "heritage",
						Operator: "NotIn",
						Values:   []string{"upmeter"},
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
}, dataplaneMetadataExporter)

// Needed to extend v1.Pod with our methods
type IstioDrivenPod v1.Pod

// Current istio revision is located in `sidecar.istio.io/status` annotation
type IstioPodStatus struct {
	Revision string `json:"revision"`
	// ... we aren't interested in the other fields
}

type IstioPodInfo struct {
	Name             string
	Namespace        string
	FullVersion      string // istio dataplane version (i.e. "1.15.6")
	Revision         string // istio dataplane revision (i.e. "v1x15")
	SpecificRevision string // istio.io/rev: vXxYZ label if it is
	InjectAnnotation bool   // sidecar.istio.io/inject annotation if it is
	InjectLabel      bool   // sidecar.istio.io/inject label if it is
}

func (p *IstioDrivenPod) getIstioCurrentRevision() string {
	var istioStatusJSON string
	var istioPodStatus IstioPodStatus
	var revision string
	var ok bool

	if istioStatusJSON, ok = p.Annotations["sidecar.istio.io/status"]; ok {
		_ = json.Unmarshal([]byte(istioStatusJSON), &istioPodStatus)

		if istioPodStatus.Revision != "" {
			revision = istioPodStatus.Revision
		} else {
			revision = istioRevsionAbsent
		}
	} else {
		revision = istioRevsionAbsent
	}
	return revision
}

func (p *IstioDrivenPod) injectAnnotation() bool {
	NeedInject := true
	if inject, ok := p.Annotations["sidecar.istio.io/inject"]; ok {
		if inject == "false" {
			NeedInject = false
		}
	}
	return NeedInject
}

func (p *IstioDrivenPod) injectLabel() bool {
	NeedInject := false
	if inject, ok := p.Labels["sidecar.istio.io/inject"]; ok {
		if inject == "true" {
			NeedInject = true
		}
	}
	return NeedInject
}

func (p *IstioDrivenPod) getIstioSpecificRevision() string {
	if specificPodRevision, ok := p.Labels["istio.io/rev"]; ok {
		return specificPodRevision
	}
	return ""
}

func (p *IstioDrivenPod) getIstioFullVersion() string {
	if istioVersion, ok := p.Annotations["istio.deckhouse.io/version"]; ok {
		return istioVersion
	} else if _, ok := p.Annotations["sidecar.istio.io/status"]; ok {
		return istioVersionUnknown
	}
	return istioVersionAbsent
}

func applyIstioPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pod := v1.Pod{}
	err := sdk.FromUnstructured(obj, &pod)
	if err != nil {
		return nil, fmt.Errorf("cannot convert pod object to pod: %v", err)
	}
	istioPod := IstioDrivenPod(pod)
	result := IstioPodInfo{
		Name:             istioPod.Name,
		Namespace:        istioPod.Namespace,
		FullVersion:      istioPod.getIstioFullVersion(),
		Revision:         istioPod.getIstioCurrentRevision(),
		SpecificRevision: istioPod.getIstioSpecificRevision(),
		InjectAnnotation: istioPod.injectAnnotation(),
		InjectLabel:      istioPod.injectLabel(),
	}

	return result, nil
}

func dataplaneMetadataExporter(input *go_hook.HookInput) error {
	if !input.Values.Get("istio.internal.globalVersion").Exists() {
		return nil
	}

	versionMap := istio_versions.VersionMapJSONToVersionMap(input.Values.Get("istio.internal.versionMap").String())
	globalRevision := versionMap[input.Values.Get("istio.internal.globalVersion").String()].Revision

	input.MetricsCollector.Expire(metadataExporterMetricsGroup)

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

		// sidecar.istio.io/inject=false annotation set -> ignore
		if !istioPodInfo.InjectAnnotation {
			continue
		}

		desiredRevision := istioRevsionAbsent

		// if label sidecar.istio.io/inject=true -> use global revision
		if istioPodInfo.InjectLabel {
			desiredRevision = globalRevision
		}
		// override if injection labels on namespace
		if desiredRevisionNS, ok := namespaceRevisionMap[istioPodInfo.Namespace]; ok {
			desiredRevision = desiredRevisionNS
		}
		// override if label istio.io/rev with specific revision exists
		if istioPodInfo.SpecificRevision != "" {
			desiredRevision = istioPodInfo.SpecificRevision
		}

		// we don't need metrics for pod without desired revision and without istio sidecar
		if desiredRevision == istioRevsionAbsent && istioPodInfo.Revision == istioRevsionAbsent {
			continue
		}

		desiredFullVersion := versionMap.GetFullVersionByRevision(desiredRevision)
		if desiredFullVersion == "" {
			desiredFullVersion = istioVersionUnknown
		}
		desiredVersion := versionMap.GetVersionByRevision(desiredRevision)
		if desiredVersion == "" {
			desiredVersion = istioVersionUnknown
		}
		var podVersion string
		if istioPodInfo.FullVersion == istioVersionAbsent {
			podVersion = istioVersionAbsent
		} else {
			podVersion = versionMap.GetVersionByFullVersion(istioPodInfo.FullVersion)
			if podVersion == "" {
				podVersion = istioVersionUnknown
			}
		}

		labels := map[string]string{
			"namespace":            istioPodInfo.Namespace,
			"dataplane_pod":        istioPodInfo.Name,
			"desired_revision":     desiredRevision,
			"revision":             istioPodInfo.Revision,
			"full_version":         istioPodInfo.FullVersion,
			"desired_full_version": desiredFullVersion,
			"version":              podVersion,
			"desired_version":      desiredVersion,
		}
		input.MetricsCollector.Set(istioPodMetadataMetricName, 1, labels, metrics.WithGroup(metadataExporterMetricsGroup))
	}
	return nil
}
