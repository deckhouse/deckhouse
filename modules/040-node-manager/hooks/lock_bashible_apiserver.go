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
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
			FilterFunc: deploymentFilterFunc,
		},
	},
}, lockHandler)

func lockHandler(input *go_hook.HookInput) error {
	snap := input.Snapshots["bashible-apiserver-deployment"]
	if len(snap) == 0 {
		return nil
	}

	valuesTag := input.Values.Get("global.modulesImages.tags.nodeManager.bashibleApiserver").String()

	deployment := snap[0].(bashibleDeployment)
	if deployment.ImageTag != valuesTag {
		annotationsPatch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]string{
					"node.deckhouse.io/bashible-locked": "true",
				},
			},
		}

		input.PatchCollector.MergePatch(annotationsPatch, "v1", "Secret", bashibleNamespace, "bashible-apiserver-context", object_patch.IgnoreMissingObject())
		return nil
	}

	if deployment.Replicas == deployment.UpdatedReplicas {
		annotationsPatch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					"node.deckhouse.io/bashible-locked": nil,
				},
			},
		}

		input.LogEntry.Errorf("Sleep started")
		time.Sleep(120 * time.Second)
		input.PatchCollector.MergePatch(annotationsPatch, "v1", "Secret", bashibleNamespace, "bashible-apiserver-context", object_patch.IgnoreMissingObject())
	}

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

	return bashibleDeployment{
		ImageTag:        deploymentImageTag,
		Replicas:        dep.Status.Replicas,
		UpdatedReplicas: dep.Status.UpdatedReplicas,
	}, nil
}

type bashibleDeployment struct {
	ImageTag        string
	Replicas        int32
	UpdatedReplicas int32
}
