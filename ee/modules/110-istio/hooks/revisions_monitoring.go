/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var (
	revisionsMonitoringMetricsGroup = "revisions"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("revisions-discovery-monitoring"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:          "namespaces_global_revision",
			ApiVersion:    "v1",
			Kind:          "Namespace",
			FilterFunc:    applyNamespaceFilter, // from revisions_discovery.go
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"istio-injection": "enabled"}},
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
	},
	Schedule: []go_hook.ScheduleConfig{ // Due to we are afraid of subscribing to all Pods in the cluster,
		{Name: "cron", Crontab: "5 * * * *"}, // we run the hook every 5 minutes to discover data-plane status.
	},
}, dependency.WithExternalDependencies(revisionsMonitoring))

type IstioDrivenPod v1.Pod
type IstioPodStatus struct {
	Revision string `json:"revision"`
	// ... we aren't interested in the other columns
}

func (p *IstioDrivenPod) getIstioRevision() string {
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

func (p *IstioDrivenPod) getProxyv2ImageTag() string {
	for _, c := range p.Spec.Containers {
		if c.Name == "istio-proxy" {
			// registry.deckhouse.io/deckhouse/ee:c0a01c0694d9490973e9079dc53d49e5ea11763dd0f0d0472d38d7d0-1650502544870
			imageSlice := strings.Split(c.Image, ":")
			return imageSlice[len(imageSlice)-1]
		}
	}
	return ""
}

func revisionsMonitoring(input *go_hook.HookInput, dc dependency.Container) error {
	// isn't discovered yet
	if !input.Values.Get("istio.internal.globalRevision").Exists() {
		return nil
	}
	if !input.Values.Get("istio.internal.revisionsToInstall").Exists() {
		return nil
	}

	input.MetricsCollector.Expire(revisionsMonitoringMetricsGroup)

	var globalRevision = input.Values.Get("istio.internal.globalRevision").String()
	var revisionsToInstall = make([]string, 0)
	var revisionsToInstallResult = input.Values.Get("istio.internal.revisionsToInstall").Array()
	for _, revisionResult := range revisionsToInstallResult {
		revisionsToInstall = append(revisionsToInstall, revisionResult.String())
	}

	var revisionSidecarImageTagMap = map[string]string{}
	for imageName, imageTagResult := range input.Values.Get("global.modulesImages.tags.istio").Map() {
		if strings.HasPrefix(imageName, "proxyv2") {
			// proxyv2V1x42 -> v1x42
			revision := strings.ToLower(strings.TrimPrefix(imageName, "proxyv2"))
			revisionSidecarImageTagMap[revision] = imageTagResult.String()
		}
	}

	var namespaceRevisionMap = map[string]string{}
	for _, ns := range append(input.Snapshots["namespaces_definite_revision"], input.Snapshots["namespaces_global_revision"]...) {
		nsInfo := ns.(NamespaceInfo)
		if nsInfo.Revision == "global" {
			namespaceRevisionMap[nsInfo.Name] = globalRevision
		} else {
			namespaceRevisionMap[nsInfo.Name] = nsInfo.Revision
		}
	}

	// check the namespaces for uninstalled desired revisions
	for ns, revision := range namespaceRevisionMap {
		if !internal.Contains(revisionsToInstall, revision) {
			// ALARM! Desired revision isn't configured to install
			labels := map[string]string{
				"namespace":        ns,
				"desired_revision": revision,
			}
			input.MetricsCollector.Set("d8_istio_desired_revision_is_not_installed", 1, labels, metrics.WithGroup(revisionsMonitoringMetricsGroup))
		}
	}

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	podList, err := k8sClient.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: "service.istio.io/canonical-name,sidecar.istio.io/inject!=false"})
	if err != nil {
		return err
	}
	for _, pod := range podList.Items {
		istioPod := IstioDrivenPod(pod)
		var desiredRevision string

		var ok bool
		if desiredRevision, ok = istioPod.Labels["istio.io/rev"]; !ok || desiredRevision == "v1x10x1" {
			// migration — delete '|| desiredRevision == "v1x10x1"' when it will be retired
			desiredRevision = ""
			if desiredRevision, ok = namespaceRevisionMap[istioPod.GetNamespace()]; !ok {
				// ALARM! The istio-driven pod has no desired revision, pod restarting will remove the sidecar
				labels := map[string]string{
					"namespace":       istioPod.GetNamespace(),
					"actual_revision": istioPod.getIstioRevision(),
				}
				input.MetricsCollector.Set("d8_istio_data_plane_without_desired_revision", 1, labels, metrics.WithGroup(revisionsMonitoringMetricsGroup))
				continue
			}
		}

		if !internal.Contains(revisionsToInstall, desiredRevision) {
			// ALARM! Desired revision isn't configured to install
			labels := map[string]string{
				"namespace":        istioPod.GetNamespace(),
				"desired_revision": desiredRevision,
			}
			input.MetricsCollector.Set("d8_istio_desired_revision_is_not_installed", 1, labels, metrics.WithGroup(revisionsMonitoringMetricsGroup))
			continue
		}

		if istioPod.getIstioRevision() != desiredRevision {
			// ALARM! The Pod's revision isn't equal the desired one, after Pod recreating, the actual revision will be changed
			labels := map[string]string{
				"namespace":        istioPod.GetNamespace(),
				"desired_revision": desiredRevision,
				"actual_revision":  istioPod.getIstioRevision(),
			}
			input.MetricsCollector.Set("d8_istio_actual_data_plane_revision_ne_desired", 1, labels, metrics.WithGroup(revisionsMonitoringMetricsGroup))
			continue
		}

		if istioPod.getProxyv2ImageTag() != revisionSidecarImageTagMap[desiredRevision] {
			// ALARM! actual sidecar minor version ne control-plane minor version
			labels := map[string]string{
				"namespace":                 istioPod.GetNamespace(),
				"revision":                  istioPod.getIstioRevision(),
				"actual_sidecar_image_tag":  istioPod.getProxyv2ImageTag(),
				"desired_sidecar_image_tag": revisionSidecarImageTagMap[desiredRevision],
			}
			input.MetricsCollector.Set("d8_istio_data_plane_patch_version_mismatch", 1, labels, metrics.WithGroup(revisionsMonitoringMetricsGroup))
			continue
		}
	}
	return nil
}
