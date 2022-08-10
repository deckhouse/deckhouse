/*
Copyright 2022 Flant JSC

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
	"errors"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

const (
	bashibleNamespace = "d8-cloud-instance-manager"
	bashibleName      = "bashible-apiserver"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/lock_bashible_apiserver",
	OnBeforeHelm: &go_hook.OrderedConfig{
		Order: 20,
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "bashible-apiserver-deployment",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{bashibleNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{bashibleName},
			},
			FilterFunc:                   deploymentFilterFunc,
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
		},
		{
			Name:       "bashible-apiserver-pods",
			ApiVersion: "v1",
			Kind:       "Pod",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{bashibleNamespace},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": bashibleName,
				},
			},
			FilterFunc: podFilterFunc,
		},
	},
}, lockHandler)

func lockHandler(input *go_hook.HookInput) error {
	snap := input.Snapshots["bashible-apiserver-deployment"]
	if len(snap) == 0 {
		return nil
	}

	valuesTag := input.Values.Get("global.modulesImages.tags.nodeManager.bashibleApiserver").String()

	deploymentTag := snap[0].(string)
	if deploymentTag != valuesTag {
		annotationsPatch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]string{
					"node.deckhouse.io/bashible-locked": "true",
				},
			},
		}

		input.PatchCollector.MergePatch(annotationsPatch, "v1", "Secret", bashibleNamespace, "bashible-apiserver-context", object_patch.IgnoreMissingObject())
	}

	for _, sn := range input.Snapshots["bashible-apiserver-pods"] {
		if sn == nil {
			continue
		}

		podImageTag := sn.(string)
		if podImageTag != valuesTag {
			return nil
		}
	}

	annotationsPatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"node.deckhouse.io/bashible-locked": nil,
			},
		},
	}

	input.PatchCollector.MergePatch(annotationsPatch, "v1", "Secret", bashibleNamespace, "bashible-apiserver-context", object_patch.IgnoreMissingObject())

	return nil
}

func deploymentFilterFunc(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var dep v1.Deployment
	err := sdk.FromUnstructured(obj, &dep)
	if err != nil {
		return nil, err
	}

	var deploymentImageTag string

	for _, cont := range dep.Spec.Template.Spec.Containers {
		if cont.Name == bashibleName {
			imageSplitIndex := strings.LastIndex(cont.Image, ":")
			if imageSplitIndex == -1 {
				return nil, errors.New("image tag not found")
			}
			deploymentImageTag = cont.Image[imageSplitIndex+1:]
		}
	}

	return deploymentImageTag, nil
}

func podFilterFunc(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod

	err := sdk.FromUnstructured(obj, &pod)
	if err != nil {
		return nil, err
	}

	var podImageTag string

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == bashibleName && !cs.Ready {
			return nil, nil
		}
	}

	for _, cont := range pod.Spec.Containers {
		if cont.Name == bashibleName {
			imageSplitIndex := strings.LastIndex(cont.Image, ":")
			if imageSplitIndex == -1 {
				return nil, errors.New("image tag not found")
			}
			podImageTag = cont.Image[imageSplitIndex+1:]
		}
	}

	return podImageTag, nil
}
