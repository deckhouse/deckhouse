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
	"context"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	gcr "github.com/google/go-containerregistry/pkg/name"
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

func lockHandler(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("bashible-apiserver-deployment")
	if len(snaps) == 0 {
		return nil
	}

	valuesDigest := input.Values.Get("global.modulesImages.digests.nodeManager.bashibleApiserver").String()
	var deployment bashibleDeployment
	err := snaps[0].UnmarshalTo(&deployment)
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'bashible-apiserver-deployment' snapshots: %w", err)
	}

	if deployment.ImageDigestOrTag != valuesDigest {
		annotationsPatch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]string{
					"node.deckhouse.io/bashible-locked": "true",
				},
			},
		}

		input.MetricsCollector.Set("d8_bashible_apiserver_locked", 1, nil)
		input.PatchCollector.PatchWithMerge(annotationsPatch, "v1", "Secret", bashibleNamespace, "bashible-apiserver-context", object_patch.WithIgnoreMissingObject())
		return nil
	}

	// track replicas count to avoid tracking Pod statuses
	if deployment.DesiredReplicas != deployment.UpdatedReplicas {
		return nil
	}

	annotationsPatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"node.deckhouse.io/bashible-locked": nil,
			},
		},
	}

	input.MetricsCollector.Set("d8_bashible_apiserver_locked", 0, nil)
	input.PatchCollector.PatchWithMerge(annotationsPatch, "v1", "Secret", bashibleNamespace, "bashible-apiserver-context", object_patch.WithIgnoreMissingObject())

	return nil
}

func deploymentFilterFunc(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var dep v1.Deployment
	err := sdk.FromUnstructured(obj, &dep)
	if err != nil {
		return nil, err
	}

	// This is because in current image can be either tag, either digest
	var deploymentImageDigestOrTag string

	for _, cont := range dep.Spec.Template.Spec.Containers {
		if cont.Name == bashibleName {
			isDigest := strings.LastIndex(cont.Image, "@sha256")
			if isDigest != -1 {
				tmpDigest, err := gcr.NewDigest(cont.Image)
				if err != nil {
					return nil, fmt.Errorf("incorrect image with digest %s in bashible apiserver", cont.Image)
				}
				deploymentImageDigestOrTag = tmpDigest.DigestStr()
			} else {
				tmpTag, err := gcr.NewTag(cont.Image)
				if err != nil {
					return nil, fmt.Errorf("incorrect image with tag %s in bashible apiserver", cont.Image)
				}
				deploymentImageDigestOrTag = tmpTag.TagStr()
			}
		}
	}

	return bashibleDeployment{
		ImageDigestOrTag: deploymentImageDigestOrTag,
		DesiredReplicas:  dep.Status.Replicas,
		UpdatedReplicas:  dep.Status.UpdatedReplicas,
	}, nil
}

type bashibleDeployment struct {
	ImageDigestOrTag string
	DesiredReplicas  int32
	UpdatedReplicas  int32
}
