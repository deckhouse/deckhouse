/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/base64"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"
)

const minimalKubernetesVersion = "1.21"

func applyDeckhousePodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var isReady bool

	pod := &v1.Pod{}
	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, fmt.Errorf("cannot parse pod object from unstructured: %v", err)
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
			isReady = true
			break
		}
	}
	return isReady, nil
}

func applySecretFilter(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}

	if err := sdk.FromUnstructured(unstructured, secret); err != nil {
		return nil, err
	}

	clusterConfigData := secret.Data["cluster-configuration.yaml"]

	var parsedClusterConfig map[string]interface{}
	if err := yaml.Unmarshal(clusterConfigData, &parsedClusterConfig); err != nil {
		return nil, err
	}

	return parsedClusterConfig, nil
}

var (
	_ = sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
		Queue:        "/modules/control-plane-manager",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:                   "DeckhousePod",
				WaitForSynchronization: pointer.BoolPtr(false),
				ExecuteHookOnEvents:    pointer.BoolPtr(false),
				ApiVersion:             "v1",
				Kind:                   "Pod",
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "app",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"deckhouse"},
						},
					},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"d8-system"},
					},
				},
				FilterFunc: applyDeckhousePodFilter,
			},
			{
				Name:       "D8ClusterConfiguration",
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cluster-configuration"},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"kube-system"},
					},
				},
				FilterFunc: applySecretFilter,
			},
		},
	}, updateClusterKubernetesVersion)
)

func updateClusterKubernetesVersion(input *go_hook.HookInput) error {
	var deckhousePodIsReady bool
	// Check if Deckhouse pod is ready
	podSnapshots := input.Snapshots["DeckhousePod"]
	if len(podSnapshots) == 0 {
		return nil
	}

	for _, podSnapshot := range podSnapshots {
		if podSnapshot == nil {
			continue
		}
		deckhousePodIsReady = podSnapshot.(bool)
		if deckhousePodIsReady {
			break
		}
	}

	if !deckhousePodIsReady {
		input.LogEntry.Info("deckhouse pod is not ready, skipping")
		return nil
	}

	desiredKubernetesVersion, err := semver.NewVersion(minimalKubernetesVersion)
	if err != nil {
		return err
	}

	secretSnapshots := input.Snapshots["D8ClusterConfiguration"]
	if len(secretSnapshots) == 0 {
		input.LogEntry.Info("cannot find kube-system/d8-cluster-configuration secret, skipping")
		return nil
	}

	secretData := secretSnapshots[0].(map[string]interface{})
	kv := secretData["kubernetesVersion"].(string)

	kubernetesVersionFromSecret, err := semver.NewVersion(kv)
	if err != nil {
		return err
	}

	if !kubernetesVersionFromSecret.LessThan(desiredKubernetesVersion) {
		return nil
	}
	resultStr := fmt.Sprintf("%d.%d", desiredKubernetesVersion.Major(), desiredKubernetesVersion.Minor())

	secretData["kubernetesVersion"] = resultStr

	s, err := yaml.Marshal(secretData)
	if err != nil {
		return nil
	}

	encoded := base64.StdEncoding.EncodeToString(s)
	patch := map[string]interface{}{
		"data": map[string]interface{}{
			"cluster-configuration.yaml": encoded,
		},
	}
	input.PatchCollector.MergePatch(patch, "v1", "Secret", "kube-system", "d8-cluster-configuration")

	return nil
}
