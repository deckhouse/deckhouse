/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
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
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
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
			Name:       "namespaces_global_revision_autoupgrade",
			ApiVersion: "v1",
			Kind:       "Namespace",
			FilterFunc: applyNamespaceFilter, // from revisions_discovery.go
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"istio-injection": "enabled", "istio.deckhouse.io/auto-upgrade": "true"},
			},
		},
		{
			Name:       "namespaces_definite_revision_autoupgrade",
			ApiVersion: "v1",
			Kind:       "Namespace",
			FilterFunc: applyNamespaceFilter, // from revisions_discovery.go
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "istio.io/rev",
						Operator: "Exists",
					},
					{
						Key:      "istio.deckhouse.io/auto-upgrade",
						Operator: "In",
						Values:   []string{"true"},
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
		{
			Name:       "deployment",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"istio.deckhouse.io/auto-upgrade": "true",
				},
			},
			FilterFunc: applyIstioDeploymentFilter,
		},
		{
			Name:       "statefulset",
			ApiVersion: "apps/v1",
			Kind:       "StatefulSet",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"istio.deckhouse.io/auto-upgrade": "true",
				},
			},
			FilterFunc: applyIstioStatefulSetFilter,
		},
		{
			Name:       "daemonset",
			ApiVersion: "apps/v1",
			Kind:       "DaemonSet",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"istio.deckhouse.io/auto-upgrade": "true",
				},
			},
			FilterFunc: applyIstioDaemonSetFilter,
		},
	},
}, dependency.WithExternalDependencies(dataplaneController))

// Needed to extend v1.Pod with our methods
type IstioDrivenPod v1.Pod

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

type IstioPodsMap map[string]map[string]bool

func NewIstioPodsMap() IstioPodsMap {
	return make(IstioPodsMap)
}

func (podsMap IstioPodsMap) add(ns, name string, podHasActualVersion bool) {
	if _, ok := podsMap[ns]; !ok {
		podsMap[ns] = make(map[string]bool)
	}
	podsMap[ns][name] = podHasActualVersion
}

func (podsMap IstioPodsMap) needToUpgrade(ns, name string) bool {
	if _, ok := podsMap[ns]; ok {
		if podHasActualVersion, ok := podsMap[ns][name]; ok {
			return !podHasActualVersion
		}
	}
	return false
}

// Needed to extend appsv1.Deployment with our methods
type IstioDrivenDeployment appsv1.Deployment

func (d *IstioDrivenDeployment) istioInjectFalseLabelOrAnnotationExists() bool {
	return istioInjectFalseExists(d.Labels) || istioInjectFalseExists(d.Annotations)
}

func (d *IstioDrivenDeployment) getInfo() IstioDeploymentInfo {
	selector, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
	if err != nil {
		return IstioDeploymentInfo{}
	}
	return IstioDeploymentInfo{
		Name:                d.Name,
		Namespace:           d.Namespace,
		UnavailableReplicas: d.Status.UnavailableReplicas,
		LabelSelector:       selector.String(),
	}
}

type IstioDeploymentInfo struct {
	Name                string
	Namespace           string
	UnavailableReplicas int32
	LabelSelector       string
}

// Needed to extend appsv1.StatefulSet with our methods
type IstioDrivenStatefulSet appsv1.StatefulSet

func (s *IstioDrivenStatefulSet) istioInjectFalseLabelOrAnnotationExists() bool {
	return istioInjectFalseExists(s.Labels) || istioInjectFalseExists(s.Annotations)
}

func (s *IstioDrivenStatefulSet) getInfo() IstioStatefulSetInfo {
	selector, err := metav1.LabelSelectorAsSelector(s.Spec.Selector)
	if err != nil {
		return IstioStatefulSetInfo{}
	}
	return IstioStatefulSetInfo{
		Name:          s.Name,
		Namespace:     s.Namespace,
		LabelSelector: selector.String(),
		Replicas:      s.Status.Replicas,
		ReadyReplicas: s.Status.ReadyReplicas,
	}
}

type IstioStatefulSetInfo struct {
	Name          string
	Namespace     string
	Replicas      int32
	ReadyReplicas int32
	LabelSelector string
}

// Needed to extend appsv1.Deployment with our methods
type IstioDrivenDaemonSet appsv1.DaemonSet

func (d *IstioDrivenDaemonSet) istioInjectFalseLabelOrAnnotationExists() bool {
	return istioInjectFalseExists(d.Labels) || istioInjectFalseExists(d.Annotations)
}

func (d *IstioDrivenDaemonSet) getInfo() IstioDaemonSetInfo {
	selector, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
	if err != nil {
		return IstioDaemonSetInfo{}
	}
	return IstioDaemonSetInfo{
		Name:              d.Name,
		Namespace:         d.Namespace,
		LabelSelector:     selector.String(),
		NumberUnavailable: d.Status.NumberUnavailable,
	}
}

type IstioDaemonSetInfo struct {
	Name              string
	Namespace         string
	NumberUnavailable int32
	LabelSelector     string
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

func applyIstioDeploymentFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	deploy := IstioDrivenDeployment{}
	err := sdk.FromUnstructured(obj, &deploy)
	if err != nil {
		return nil, fmt.Errorf("cannot convert deployment object to istio driven deployment: %v", err)
	}
	return deploy.getInfo(), nil
}

func applyIstioStatefulSetFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	sts := IstioDrivenStatefulSet{}
	err := sdk.FromUnstructured(obj, &sts)
	if err != nil {
		return nil, fmt.Errorf("cannot convert statefulset object to istio driven statefulset: %v", err)
	}
	return sts.getInfo(), nil
}

func applyIstioDaemonSetFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ds := IstioDrivenDaemonSet{}
	err := sdk.FromUnstructured(obj, &ds)
	if err != nil {
		return nil, fmt.Errorf("cannot convert daemonset object to istio driven daemonset: %v", err)
	}
	return ds.getInfo(), nil
}

func istioInjectFalseExists(m map[string]string) bool {
	if inject, ok := m["sidecar.istio.io/inject"]; ok {
		if inject == "false" {
			return true
		}
	}
	return false
}

func dataplaneController(input *go_hook.HookInput, dc dependency.Container) error {
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

	istioPodsMap := NewIstioPodsMap()

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
		istioPodsMap.add(istioPodInfo.Namespace, istioPodInfo.Name, istioPodInfo.FullVersion == desiredFullVersion)
	}

	k8s, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	for _, ns := range append(input.Snapshots["namespaces_global_revision_autoupgrade"], input.Snapshots["namespaces_definite_revision_autoupgrade"]...) {
		nsInfo := ns.(NamespaceInfo)

		deployments, _ := k8s.AppsV1().Deployments(nsInfo.Name).List(context.TODO(), metav1.ListOptions{})
		for _, deploy := range deployments.Items {
			istioDrivenDeploy := IstioDrivenDeployment(deploy)
			if !istioDrivenDeploy.istioInjectFalseLabelOrAnnotationExists() {
				input.Snapshots["deployment"] = append(input.Snapshots["deployment"], istioDrivenDeploy.getInfo())
			}
		}

		statefulSets, _ := k8s.AppsV1().StatefulSets(nsInfo.Name).List(context.TODO(), metav1.ListOptions{})
		for _, sts := range statefulSets.Items {
			istioDrivenSts := IstioDrivenStatefulSet(sts)
			if !istioDrivenSts.istioInjectFalseLabelOrAnnotationExists() {
				input.Snapshots["statefulset"] = append(input.Snapshots["statefulset"], istioDrivenSts.getInfo())
			}
		}

		daemonSets, _ := k8s.AppsV1().DaemonSets(nsInfo.Name).List(context.TODO(), metav1.ListOptions{})
		for _, ds := range daemonSets.Items {
			istioDrivenDs := IstioDrivenDaemonSet(ds)
			if !istioDrivenDs.istioInjectFalseLabelOrAnnotationExists() {
				input.Snapshots["daemonset"] = append(input.Snapshots["daemonset"], istioDrivenDs.getInfo())
			}
		}
	}

	upgradeIstioDeployment(input, k8s, istioPodsMap)
	upgradeIstioStatefulSet(input, k8s, istioPodsMap)
	upgradeIstioDaemonSet(input, k8s, istioPodsMap)

	return nil
}

func upgradeIstioDeployment(input *go_hook.HookInput, k8s k8s.Client, istioPodsMap IstioPodsMap) {
	for _, deployRaw := range input.Snapshots["deployment"] {
		deployInfo := deployRaw.(IstioDeploymentInfo)
		if deployInfo.UnavailableReplicas != 0 {
			continue
		}
		replicaSets, _ := k8s.AppsV1().ReplicaSets(deployInfo.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: deployInfo.LabelSelector})
		for _, rs := range replicaSets.Items {
			rsSelector, err := metav1.LabelSelectorAsSelector(rs.Spec.Selector)
			if err != nil {
				input.LogEntry.Error(err)
			}
			pods, _ := k8s.CoreV1().Pods(deployInfo.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: rsSelector.String()})
			for _, pod := range pods.Items {
				if istioPodsMap.needToUpgrade(pod.Namespace, pod.Name) {
					err = k8s.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
					if err != nil {
						input.LogEntry.Error(err)
					}
					break // kill only one pod per iteration
				}
			}
		}
	}
}

func upgradeIstioStatefulSet(input *go_hook.HookInput, k8s k8s.Client, istioPodsMap IstioPodsMap) {
	for _, stsRaw := range input.Snapshots["statefulset"] {
		sts := stsRaw.(IstioStatefulSetInfo)
		if sts.Replicas != sts.ReadyReplicas {
			continue
		}
		pods, _ := k8s.CoreV1().Pods(sts.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: sts.LabelSelector})
		for _, pod := range pods.Items {
			if istioPodsMap.needToUpgrade(pod.Namespace, pod.Name) {
				err := k8s.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
				if err != nil {
					input.LogEntry.Error(err)
				}
				break // kill only one pod per iteration
			}
		}
	}
}

func upgradeIstioDaemonSet(input *go_hook.HookInput, k8s k8s.Client, istioPodsMap IstioPodsMap) {
	for _, dsRaw := range input.Snapshots["daemonset"] {
		ds := dsRaw.(IstioDaemonSetInfo)
		if ds.NumberUnavailable != 0 {
			continue
		}
		pods, _ := k8s.CoreV1().Pods(ds.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: ds.LabelSelector})
		for _, pod := range pods.Items {
			if istioPodsMap.needToUpgrade(pod.Namespace, pod.Name) {
				err := k8s.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
				if err != nil {
					input.LogEntry.Error(err)
				}
				break // kill only one pod per iteration
			}
		}
	}
}
