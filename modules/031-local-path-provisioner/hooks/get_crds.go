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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/031-local-path-provisioner/hooks/internal/v1alpha1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/local-path-provisioner",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "lpp",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "LocalPathProvisioner",
			FilterFunc: getLPPCRDFilter,
		},
	},
}, getLPPCRDsHandler)

type LocalPathProvisionerInfo struct {
	Name string                            `json:"name"`
	Spec v1alpha1.LocalPathProvisionerSpec `json:"spec"`
}

func getLPPCRDFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	lpp := new(v1alpha1.LocalPathProvisioner)
	err := sdk.FromUnstructured(obj, lpp)
	if err != nil {
		return nil, err
	}

	return LocalPathProvisionerInfo{
		Name: lpp.Name,
		Spec: lpp.Spec,
	}, nil
}

func getLPPCRDsHandler(_ context.Context, input *go_hook.HookInput) error {
	localPathProvisioners, err := sdkobjectpatch.UnmarshalToStruct[LocalPathProvisionerInfo](input.Snapshots, "lpp")
	if err != nil {
		return fmt.Errorf("failed to unmarshal lpp snapshot: %w", err)
	}
	if len(localPathProvisioners) == 0 {
		localPathProvisioners = make([]LocalPathProvisionerInfo, 0)
	}

	input.Values.Set("localPathProvisioner.internal.localPathProvisioners", localPathProvisioners)
	return nil
}
