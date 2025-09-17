/*
Copyright 2021 Flant JSC

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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	gcr "github.com/google/go-containerregistry/pkg/name"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

func getDeploymentImage(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	deployment := &appsv1.Deployment{}
	err := sdk.FromUnstructured(obj, deployment)
	if err != nil {
		return nil, fmt.Errorf("cannot convert deckhouse deployment to deployment: %v", err)
	}

	return deployment.Spec.Template.Spec.Containers[0].Image, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "deckhouse",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse"},
			},
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          getDeploymentImage,
		},
	},
}, parseDeckhouseImage)

func parseDeckhouseImage(_ context.Context, input *go_hook.HookInput) error {
	const (
		deckhouseImagePath = "deckhouse.internal.currentReleaseImageName"
		deckhouseBasePath  = "global.modulesImages.registry.base"
	)

	deckhouseImages, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, "deckhouse")
	if err != nil {
		return fmt.Errorf("failed to unmarshal deckhouse snapshot: %w", err)
	}

	if len(deckhouseImages) != 1 {
		return fmt.Errorf("deckhouse was not able to find an image of itself")
	}
	image := deckhouseImages[0]

	imageRepoTag, err := gcr.NewTag(image)
	if err != nil {
		return fmt.Errorf("incorrect image: %s", image)
	}
	tag := imageRepoTag.TagStr()

	// Set deckhouse image only if it was not set before, e.g. by stabilize_release_channel hook
	if input.Values.Get(deckhouseImagePath).String() == "" {
		if !input.Values.Get(deckhouseBasePath).Exists() {
			return fmt.Errorf("registry base path doesn't exist yet")
		}
		base := input.Values.Get(deckhouseBasePath).String()
		input.Values.Set(deckhouseImagePath, fmt.Sprintf("%s:%s", base, tag))
	}

	return nil
}
