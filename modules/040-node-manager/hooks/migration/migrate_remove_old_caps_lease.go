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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	d8CapsNs           = "d8-cloud-instance-manager"
	d8CapsLeaseNameOld = "faf94607.cluster.x-k8s.io"
)

// TODO: Remove this hook after 1.76.0+ release
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "remove_old_caps_lease",
			ApiVersion: "coordination.k8s.io/v1",
			Kind:       "Lease",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{
					d8CapsNs,
				}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				d8CapsLeaseNameOld,
			}},
			FilterFunc: applyCapsLeaseFilter,
		},
	},
}, dependency.WithExternalDependencies(removeOldCapsLease))

func removeOldCapsLease(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	kubeClient := dc.MustGetK8sClient()

	err := kubeClient.CoordinationV1().Leases(d8CapsNs).Delete(context.Background(), d8CapsLeaseNameOld, metav1.DeleteOptions{})

	if err != nil && !errors.IsNotFound(err) {
		input.Logger.Info(err.Error())
	}

	return nil
}

func applyCapsLeaseFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var lease = &coordinationv1.Lease{}
	err := sdk.FromUnstructured(obj, lease)
	if err != nil {
		return nil, fmt.Errorf("cannot convert lease from unstructured: %v", err)
	}

	return lease, nil
}
