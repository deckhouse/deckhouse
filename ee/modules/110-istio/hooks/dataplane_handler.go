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
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal/istio_versions"
	"github.com/deckhouse/deckhouse/go_lib/telemetry"
)

const (
	istioRevsionAbsent           = "absent"
	istioVersionAbsent           = "absent"
	istioVersionUnknown          = "unknown"
	istioPodMetadataMetricName   = "d8_istio_dataplane_metadata"
	metadataExporterMetricsGroup = "metadata"
	autoUpgradeLabelName         = "istio.deckhouse.io/auto-upgrade"
	patchTemplate                = `{ "spec": { "template": { "metadata": { "annotations": { "istio.deckhouse.io/full-version": "%s" } } } } }`
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("dataplane-handler"),
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
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
		},
		{
			Name:       "istio_pod",
			ApiVersion: "v1",
			Kind:       "Pod",
			FilterFunc: applyIstioDrivenPodFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "job-name",
						Operator: metav1.LabelSelectorOpDoesNotExist,
					},
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpNotIn,
						Values:   []string{"upmeter"},
					},
					{
						Key:      "sidecar.istio.io/inject",
						Operator: metav1.LabelSelectorOpNotIn,
						Values:   []string{"false"},
					},
				},
			},
		},
		{
			Name:       "deployment",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			FilterFunc: applyDeploymentFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpNotIn,
						Values:   []string{"upmeter", "deckhouse"},
					},
				},
			},
		},
		{
			Name:       "daemonset",
			ApiVersion: "apps/v1",
			Kind:       "DaemonSet",
			FilterFunc: applyDaemonSetFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpNotIn,
						Values:   []string{"upmeter", "deckhouse"},
					},
				},
			},
		},
		{
			Name:       "statefulset",
			ApiVersion: "apps/v1",
			Kind:       "StatefulSet",
			FilterFunc: applyStatefulSetFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpNotIn,
						Values:   []string{"upmeter", "deckhouse"},
					},
				},
			},
		},
		{
			Name:       "replicaset",
			ApiVersion: "apps/v1",
			Kind:       "ReplicaSet",
			FilterFunc: applyReplicaSetFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpNotIn,
						Values:   []string{"upmeter", "deckhouse"},
					},
				},
			},
		},
	},
}, dataplaneHandler)

// Needed to extend v1.Pod with our methods
type IstioDrivenPod v1.Pod

// Current istio revision is located in `sidecar.istio.io/status` annotation
type IstioPodStatus struct {
	Revision string `json:"revision"`
	// ... we aren't interested in the other fields
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
	if istioVersion, ok := p.Annotations["istio.deckhouse.io/full-version"]; ok {
		return istioVersion
	} else if _, ok := p.Annotations["sidecar.istio.io/status"]; ok {
		return istioVersionUnknown
	}
	return istioVersionAbsent
}

type Owner struct {
	Name string
	Kind string
}

type upgradeCandidate struct {
	kind                              string
	name                              string
	namespace                         string
	specTemplateAnnotationFullVersion string
	desiredFullVersion                string
	isReady                           bool
	needUpgrade                       bool
}

type upgradeCandidateRS struct {
	owner Owner
}

type IstioDrivenPodFilterResult struct {
	Name             string
	Namespace        string
	FullVersion      string // istio dataplane version (i.e. "1.15.6")
	Revision         string // istio dataplane revision (i.e. "v1x15")
	SpecificRevision string // istio.io/rev: vXxYZ label if it is
	InjectAnnotation bool   // sidecar.istio.io/inject annotation if it is
	InjectLabel      bool   // sidecar.istio.io/inject label if it is
	Owner            Owner
}

func applyIstioDrivenPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pod := v1.Pod{}
	err := sdk.FromUnstructured(obj, &pod)
	if err != nil {
		return nil, fmt.Errorf("cannot convert pod object to pod: %v", err)
	}
	istioPod := IstioDrivenPod(pod)

	result := IstioDrivenPodFilterResult{
		Name:             istioPod.Name,
		Namespace:        istioPod.Namespace,
		FullVersion:      istioPod.getIstioFullVersion(),
		Revision:         istioPod.getIstioCurrentRevision(),
		SpecificRevision: istioPod.getIstioSpecificRevision(),
		InjectAnnotation: istioPod.injectAnnotation(),
		InjectLabel:      istioPod.injectLabel(),
	}

	if len(pod.OwnerReferences) == 1 {
		result.Owner.Name = pod.OwnerReferences[0].Name
		result.Owner.Kind = pod.OwnerReferences[0].Kind
	}
	return result, nil
}

type K8SControllerFilterResult struct {
	Name                              string
	Kind                              string
	Namespace                         string
	IsReady                           bool   // if the controller is ready
	AutoUpgradeLabelExists            bool   // the label can be installed either on the controller or on the namespace
	SpecTemplateAnnotationFullVersion string // value of .spec.template.annotations["istio.deckhouse.io/full-version"]
	Owner                             Owner
}

func applyDeploymentFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	deploy := appsv1.Deployment{}
	err := sdk.FromUnstructured(obj, &deploy)
	if err != nil {
		return nil, fmt.Errorf("cannot convert deployment object to deployment: %v", err)
	}

	specTemplateAnnotationFullVersion := ""
	if a, ok := deploy.Spec.Template.Annotations["istio.deckhouse.io/full-version"]; ok {
		specTemplateAnnotationFullVersion = a
	}

	result := K8SControllerFilterResult{
		Name:                              deploy.Name,
		Kind:                              deploy.Kind,
		Namespace:                         deploy.Namespace,
		IsReady:                           deploy.Status.UnavailableReplicas == 0,
		SpecTemplateAnnotationFullVersion: specTemplateAnnotationFullVersion,
	}

	if _, ok := deploy.Labels[autoUpgradeLabelName]; ok {
		result.AutoUpgradeLabelExists = deploy.Labels[autoUpgradeLabelName] == "true"
	}

	return result, nil
}

func applyStatefulSetFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	sts := appsv1.StatefulSet{}
	err := sdk.FromUnstructured(obj, &sts)
	if err != nil {
		return nil, fmt.Errorf("cannot convert statefulset object to statefulset: %v", err)
	}

	specTemplateAnnotationFullVersion := ""
	if a, ok := sts.Spec.Template.Annotations["istio.deckhouse.io/full-version"]; ok {
		specTemplateAnnotationFullVersion = a
	}

	result := K8SControllerFilterResult{
		Name:                              sts.Name,
		Kind:                              sts.Kind,
		Namespace:                         sts.Namespace,
		IsReady:                           sts.Status.Replicas == sts.Status.ReadyReplicas,
		SpecTemplateAnnotationFullVersion: specTemplateAnnotationFullVersion,
	}

	if _, ok := sts.Labels[autoUpgradeLabelName]; ok {
		result.AutoUpgradeLabelExists = sts.Labels[autoUpgradeLabelName] == "true"
	}

	return result, nil
}

func applyDaemonSetFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ds := appsv1.DaemonSet{}
	err := sdk.FromUnstructured(obj, &ds)
	if err != nil {
		return nil, fmt.Errorf("cannot convert deployment object to deployment: %v", err)
	}

	specTemplateAnnotationFullVersion := ""
	if a, ok := ds.Spec.Template.Annotations["istio.deckhouse.io/full-version"]; ok {
		specTemplateAnnotationFullVersion = a
	}

	result := K8SControllerFilterResult{
		Name:                              ds.Name,
		Kind:                              ds.Kind,
		Namespace:                         ds.Namespace,
		IsReady:                           ds.Status.NumberUnavailable == 0,
		SpecTemplateAnnotationFullVersion: specTemplateAnnotationFullVersion,
	}

	if _, ok := ds.Labels[autoUpgradeLabelName]; ok {
		result.AutoUpgradeLabelExists = ds.Labels[autoUpgradeLabelName] == "true"
	}

	return result, nil
}

func applyReplicaSetFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	rs := appsv1.ReplicaSet{}
	err := sdk.FromUnstructured(obj, &rs)
	if err != nil {
		return nil, fmt.Errorf("cannot convert replicaset object to replicaset: %v", err)
	}

	result := K8SControllerFilterResult{
		Name:      rs.Name,
		Namespace: rs.Namespace,
		IsReady:   rs.Status.Replicas == rs.Status.ReadyReplicas,
	}

	if len(rs.OwnerReferences) == 1 {
		result.Owner.Name = rs.OwnerReferences[0].Name
		result.Owner.Kind = rs.OwnerReferences[0].Kind
	}

	return result, nil
}

func dataplaneHandler(input *go_hook.HookInput) error {
	if !input.Values.Get("istio.internal.globalVersion").Exists() {
		return nil
	}

	versionMap := istio_versions.VersionMapJSONToVersionMap(input.Values.Get("istio.internal.versionMap").String())

	globalRevision := versionMap[input.Values.Get("istio.internal.globalVersion").String()].Revision

	input.MetricsCollector.Expire(metadataExporterMetricsGroup)

	// create istio namespace map to find out needed revisions and versions
	istioNamespaceMap := make(map[string]IstioNamespaceFilterResult)
	for _, ns := range append(input.Snapshots["namespaces_definite_revision"], input.Snapshots["namespaces_global_revision"]...) {
		nsInfo := ns.(IstioNamespaceFilterResult)
		if nsInfo.RevisionRaw == "global" {
			nsInfo.Revision = globalRevision
		} else {
			nsInfo.Revision = nsInfo.RevisionRaw
		}
		istioNamespaceMap[nsInfo.Name] = nsInfo
	}

	// controllers are potential candidates for updating sidecar versions
	upgradeCandidates := make([]*upgradeCandidate, 0)

	// index for upgradeCandidates
	// upgradeCandidatesMap[kind][namespace][name]*upgradeCandidate{}
	upgradeCandidatesMap := make(map[string]map[string]map[string]*upgradeCandidate)

	k8sControllers := make([]go_hook.FilterResult, 0)
	k8sControllers = append(k8sControllers, input.Snapshots["deployment"]...)
	k8sControllers = append(k8sControllers, input.Snapshots["statefulset"]...)
	k8sControllers = append(k8sControllers, input.Snapshots["daemonset"]...)

	// fill in upgradeCandidates and upgradeCandidatesMap
	for _, k8sControllerRaw := range k8sControllers {
		k8sController := k8sControllerRaw.(K8SControllerFilterResult)

		// check if AutoUpgrade Label Exists on namespace
		var namespaceAutoUpgradeLabelExists bool
		if k8sControllerNS, ok := istioNamespaceMap[k8sController.Namespace]; ok {
			namespaceAutoUpgradeLabelExists = k8sControllerNS.AutoUpgradeLabelExists
		}

		// if an istio.deckhouse.io/auto-upgrade Label exists in the namespace or in the controller -> add to upgradeCandidates, upgradeCandidatesMap
		if namespaceAutoUpgradeLabelExists || k8sController.AutoUpgradeLabelExists {
			uc := &upgradeCandidate{
				kind:                              k8sController.Kind,
				name:                              k8sController.Name,
				namespace:                         k8sController.Namespace,
				isReady:                           k8sController.IsReady,
				specTemplateAnnotationFullVersion: k8sController.SpecTemplateAnnotationFullVersion,
			}
			upgradeCandidates = append(upgradeCandidates, uc)
			if _, ok := upgradeCandidatesMap[k8sController.Kind]; !ok {
				upgradeCandidatesMap[k8sController.Kind] = make(map[string]map[string]*upgradeCandidate)
			}
			if _, ok := upgradeCandidatesMap[k8sController.Kind][k8sController.Namespace]; !ok {
				upgradeCandidatesMap[k8sController.Kind][k8sController.Namespace] = make(map[string]*upgradeCandidate)
			}
			// add pointer to last added candidate
			upgradeCandidatesMap[k8sController.Kind][k8sController.Namespace][k8sController.Name] = uc
		}
	}

	// replicaSets[namespace][replicaset-name]upgradeCandidateRS
	replicaSets := make(map[string]map[string]upgradeCandidateRS)

	// create a map of the replica sets depending on the deployments from upgradeCandidatesMap map
	for _, rs := range input.Snapshots["replicaset"] {
		rsInfo := rs.(K8SControllerFilterResult)
		if rsInfo.Owner.Kind == "Deployment" {
			if _, ok := upgradeCandidatesMap["Deployment"][rsInfo.Namespace][rsInfo.Owner.Name]; ok {
				if _, ok := replicaSets[rsInfo.Namespace]; !ok {
					replicaSets[rsInfo.Namespace] = make(map[string]upgradeCandidateRS)
				}
				replicaSets[rsInfo.Namespace][rsInfo.Name] = upgradeCandidateRS{
					owner: Owner{
						Kind: rsInfo.Owner.Kind,
						Name: rsInfo.Owner.Name,
					},
				}
			}
		}
	}

	var istioDrivenPodsCount float64
	podsByFullVersion := make(map[string]float64)

	// map of namespace, which will be ignored when selecting controllers to update
	ignoredNamespace := make(map[string]struct{})

	for _, pod := range input.Snapshots["istio_pod"] {
		istioPod := pod.(IstioDrivenPodFilterResult)

		// sidecar.istio.io/inject=false annotation set -> ignore
		if !istioPod.InjectAnnotation {
			continue
		}

		desiredRevision := istioRevsionAbsent

		// if label sidecar.istio.io/inject=true -> use global revision
		if istioPod.InjectLabel {
			desiredRevision = globalRevision
		}
		// override if injection labels on namespace
		if desiredRevisionNS, ok := istioNamespaceMap[istioPod.Namespace]; ok {
			desiredRevision = desiredRevisionNS.Revision
		}
		// override if label istio.io/rev with specific revision exists
		if istioPod.SpecificRevision != "" {
			desiredRevision = istioPod.SpecificRevision
		}

		// we don't need metrics for pod without desired revision and without istio sidecar
		if desiredRevision == istioRevsionAbsent && istioPod.Revision == istioRevsionAbsent {
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
		if istioPod.FullVersion == istioVersionAbsent {
			podVersion = istioVersionAbsent
		} else {
			podVersion = versionMap.GetVersionByFullVersion(istioPod.FullVersion)
			if podVersion == "" {
				podVersion = istioVersionUnknown
			}
		}

		labels := map[string]string{
			"namespace":            istioPod.Namespace,
			"dataplane_pod":        istioPod.Name,
			"desired_revision":     desiredRevision,
			"revision":             istioPod.Revision,
			"full_version":         istioPod.FullVersion,
			"desired_full_version": desiredFullVersion,
			"version":              podVersion,
			"desired_version":      desiredVersion,
		}

		input.MetricsCollector.Set(istioPodMetadataMetricName, 1, labels, metrics.WithGroup(metadataExporterMetricsGroup))

		// search for k8sControllers that require a sidecar update
		if istioPod.FullVersion != desiredFullVersion {
			switch istioPod.Owner.Kind {
			case "ReplicaSet":
				if rs, ok := replicaSets[istioPod.Namespace][istioPod.Owner.Name]; ok {
					// if owner of replica set exists -> process it
					if _, ok := upgradeCandidatesMap[rs.owner.Kind][istioPod.Namespace][rs.owner.Name]; ok {
						upgradeCandidatesMap[rs.owner.Kind][istioPod.Namespace][rs.owner.Name].needUpgrade = true
						upgradeCandidatesMap[rs.owner.Kind][istioPod.Namespace][rs.owner.Name].desiredFullVersion = desiredFullVersion

						c := upgradeCandidatesMap[rs.owner.Kind][istioPod.Namespace][rs.owner.Name]
						// if controller is not ready and desired full version annotation already exists, so controller is updating -> namespace will be skipped for upgrade
						if !c.isReady && c.specTemplateAnnotationFullVersion == c.desiredFullVersion {
							ignoredNamespace[istioPod.Namespace] = struct{}{}
						}
					}
				}
			case "StatefulSet", "DaemonSet":
				if _, ok := upgradeCandidatesMap[istioPod.Owner.Kind][istioPod.Namespace][istioPod.Owner.Name]; ok {
					upgradeCandidatesMap[istioPod.Owner.Kind][istioPod.Namespace][istioPod.Owner.Name].needUpgrade = true
					upgradeCandidatesMap[istioPod.Owner.Kind][istioPod.Namespace][istioPod.Owner.Name].desiredFullVersion = desiredFullVersion

					c := upgradeCandidatesMap[istioPod.Owner.Kind][istioPod.Namespace][istioPod.Owner.Name]
					// if controller is not ready and desired full version annotation already exists, so controller is updating -> namespace will be skipped for upgrade
					if !c.isReady && c.specTemplateAnnotationFullVersion == c.desiredFullVersion {
						ignoredNamespace[istioPod.Namespace] = struct{}{}
					}
				}
			}
		}

		// istio telemetry stats
		istioDrivenPodsCount++
		podsByFullVersion[istioPod.FullVersion]++
	}

	// istio telemetry
	input.MetricsCollector.Set(telemetry.WrapName("istio_driven_pods_total"), istioDrivenPodsCount, nil)
	for v, c := range podsByFullVersion {
		input.MetricsCollector.Set(telemetry.WrapName("istio_driven_pods_group_by_full_version_total"), c, map[string]string{
			"full_version": v,
		})
	}

	// go through the whole list of candidates and patch the controller where required
	for _, candidate := range upgradeCandidates {
		// some controllers in this namespace are in the process of updating now -> skip namespace
		if _, ok := ignoredNamespace[candidate.namespace]; ok {
			continue
		}
		if candidate.needUpgrade && candidate.isReady {
			input.LogEntry.Infof("Patch %s '%s' in namespace '%s' with full version '%s'", candidate.kind, candidate.name, candidate.namespace, candidate.desiredFullVersion)
			input.PatchCollector.MergePatch(fmt.Sprintf(patchTemplate, candidate.desiredFullVersion), "apps/v1", candidate.kind, candidate.namespace, candidate.name)
			// skip this namespace on next iteration
			ignoredNamespace[candidate.namespace] = struct{}{}
		}
	}
	return nil
}
