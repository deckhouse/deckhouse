/*
Copyright 2025 Flant JSC

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
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 5 * time.Second,
		ExecutionBurst:       3,
	},
	Queue: "/modules/deckhouse/disable-default-sa-token-automount",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "default-sa",
			ApiVersion:                   "v1",
			Kind:                         "ServiceAccount",
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(true),
			NamespaceSelector: &types.NamespaceSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "heritage",
							Operator: metav1.LabelSelectorOpIn,
							Values: []string{
								"deckhouse",
							},
						},
					},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"default"},
			},
			FilterFunc: applySAFilter,
		},
	},
}, dependency.WithExternalDependencies(disableDefaultSATokenAutomount))

type SA struct {
	Name                         string
	Namespace                    string
	AutomountServiceAccountToken bool
}

func applySAFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sa v1.ServiceAccount

	err := sdk.FromUnstructured(obj, &sa)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	if sa.AutomountServiceAccountToken == nil {
		sa.AutomountServiceAccountToken = ptr.To(true)
	}

	return &SA{
		Name:                         sa.Name,
		Namespace:                    sa.Namespace,
		AutomountServiceAccountToken: *sa.AutomountServiceAccountToken,
	}, nil
}

func updateSA(k8 k8s.Client, sa *SA) error {
	s := &v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
		AutomountServiceAccountToken: ptr.To(false),
	}

	if _, err := k8.CoreV1().ServiceAccounts(sa.Namespace).Update(context.TODO(), s, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

func disableDefaultSATokenAutomount(input *go_hook.HookInput, dc dependency.Container) error {
	sa := input.Snapshots["default-sa"]

	k8, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("can't init Kubernetes client: %v", err)
	}

	for _, s := range sa {
		if s.(SA).AutomountServiceAccountToken {
			err = updateSA(k8, s.(*SA))
			if err != nil {
				return fmt.Errorf("can't update ServiceAccount: %v", err)
			}
		}
	}

	return nil
}
