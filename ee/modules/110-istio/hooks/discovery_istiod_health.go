/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal/istio_versions"
)

const isGlobalVersionIstiodReadyPath = "istio.internal.isGlobalVersionIstiodReady"

type istiodPod struct {
	Name     string
	Revision string
	Phase    v1.PodPhase
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "disocvery_istiod_health",
			Crontab: "*/5 * * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "istiod_pods",
			ApiVersion:        "v1",
			Kind:              "Pod",
			NamespaceSelector: internal.NsSelector(),
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "istio.io/rev",
						Operator: metav1.LabelSelectorOpExists,
					},
					{
						Key:      "app",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"istiod"},
					},
				},
			},
			FilterFunc: applyIstiodPodFilter,
		},
		{
			Name:       "istio_sidecar_injector_global_webhook",
			ApiVersion: "admissionregistration.k8s.io/v1",
			Kind:       "MutatingWebhookConfiguration",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-istio-sidecar-injector-global"},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "module",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"istio"},
					},
					{
						Key:      "istio.deckhouse.io/full-version",
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc: applyIstiodInjectorWebhook,
		},
		// we need to subscribe to the istio module configuration to update the istiod isReady status in time
		{
			Name:       "istio_module_config",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"istio"},
			},
			FilterFunc: applyIstioModuleConfig,
		},
	},
}, discoveryIstiodHealthHook)

func applyIstiodPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod v1.Pod
	err := sdk.FromUnstructured(obj, &pod)
	if err != nil {
		return nil, err
	}
	return istiodPod{
		Name:     pod.Name,
		Revision: pod.Labels["istio.io/rev"],
		Phase:    pod.Status.Phase,
	}, nil
}

func applyIstiodInjectorWebhook(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	labels := obj.GetLabels()
	fullVer := labels["istio.deckhouse.io/full-version"]
	return fullVer, nil
}

func applyIstioModuleConfig(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func discoveryIstiodHealthHook(input *go_hook.HookInput) error {
	var isGlobalVersionIstiodReady bool
	var injectorGlobalFullVer string
	if !input.Values.Get("istio.internal.globalVersion").Exists() {
		return nil
	}

	if len(input.Snapshots["istio_sidecar_injector_global_webhook"]) == 1 {
		injectorGlobalFullVer = input.Snapshots["istio_sidecar_injector_global_webhook"][0].(string)
	}

	versionMap := istio_versions.VersionMapJSONToVersionMap(input.Values.Get("istio.internal.versionMap").String())
	globalVersion := input.Values.Get("istio.internal.globalVersion").String()
	globalFullVersion := versionMap[globalVersion].FullVersion

	// reset istiod ready statuses
	for ver, istioInfo := range versionMap {
		istioInfo.IsReady = false
		versionMap[ver] = istioInfo
	}

	for _, podRaw := range input.Snapshots["istiod_pods"] {
		pod := podRaw.(istiodPod)
		podVer := versionMap.GetVersionByRevision(pod.Revision)
		podFullVer := versionMap.GetFullVersionByRevision(pod.Revision)
		// health cheks for istiod
		// if istio revision pod is running and istiod revision is not global -> ready
		// if istio revision pod is running and istiod revision is global and injector webhook has actual full version -> ready
		if pod.Phase == v1.PodRunning {
			if podVer != globalVersion {
				versionMap.SetRevisionStatus(pod.Revision, true)
			} else {
				if podFullVer == globalFullVersion && injectorGlobalFullVer == globalFullVersion {
					versionMap.SetRevisionStatus(pod.Revision, true)
				} else {
					versionMap.SetRevisionStatus(pod.Revision, false)
				}
			}
			if versionMap.GetVersionByRevision(pod.Revision) == globalVersion {
				isGlobalVersionIstiodReady = true
			}
		} else {
			versionMap.SetRevisionStatus(pod.Revision, false)
		}
	}

	input.Values.Set(isGlobalVersionIstiodReadyPath, isGlobalVersionIstiodReady)
	input.Values.Set(versionMapPath, versionMap)
	if !isGlobalVersionIstiodReady {
		// There is a problem deleting the webhook configuration from helm. It must be deleted in the first place.
		input.PatchCollector.Delete("admissionregistration.k8s.io/v1", "ValidatingWebhookConfiguration", "", "d8-istio-validator-global", object_patch.InForeground())
	}
	return nil
}
