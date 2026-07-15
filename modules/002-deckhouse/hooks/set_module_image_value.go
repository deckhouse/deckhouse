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
	"log/slog"

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

	var desired string
	switch len(deckhouseImages) {
	case 1:
		image := deckhouseImages[0]
		imageRepoTag, err := gcr.NewTag(image)
		if err != nil {
			return fmt.Errorf("incorrect image: %s", image)
		}
		tag := imageRepoTag.TagStr()

		if !input.Values.Get(deckhouseBasePath).Exists() {
			return fmt.Errorf("registry base path doesn't exist yet")
		}
		base := input.Values.Get(deckhouseBasePath).String()
		desired = fmt.Sprintf("%s:%s", base, tag)
	case 0:
		// Deckhouse is not self-hosted in this cluster (e.g. it runs in a
		// parent cluster and manages this one via a kubeconfig): there is no
		// own Deployment to read the image from.
		// TODO(vcp): temporary hardcode; decide on the real image source for
		// the not-self-hosted mode.
		desired = notSelfHostedDeckhouseImage
		input.Logger.Info("deckhouse Deployment not found, using the hardcoded image", slog.String("image", desired))
	default:
		return fmt.Errorf("deckhouse was not able to find an image of itself")
	}

	// Guards a race with bumpDeckhouseDeployment: if the hook re-runs on the
	// old leader between the Deployment bump and leader handover, a stale
	// values entry would make Helm roll the Deployment back.
	if input.Values.Get(deckhouseImagePath).String() != desired {
		input.Values.Set(deckhouseImagePath, desired)
	}

	return nil
}

// notSelfHostedDeckhouseImage is the deckhouse image used when there is no
// own Deployment to read the image from (not-self-hosted mode).
// TODO(vcp): temporary hardcode; decide on the real image source.
const notSelfHostedDeckhouseImage = "dev-registry.deckhouse.io/sys/deckhouse-oss:pr21346"
