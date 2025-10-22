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
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:       "/modules/istio/cni_daemonset",
	OnAfterHelm: &go_hook.OrderedConfig{},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "daemonset",
			ApiVersion: "apps/v1",
			Kind:       "DaemonSet",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-istio"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"istio-cni-node"},
			},
			FilterFunc: hostNetworkFalse,
		},
	},
}, dependency.WithExternalDependencies(handleHostNetworkFalseWithHostPorts))

func hostNetworkFalse(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ds := &appsv1.DaemonSet{}

	err := sdk.FromUnstructured(obj, ds)
	if err != nil {
		return nil, err
	}

	if !ds.Spec.Template.Spec.HostNetwork {
		return true, nil
	}

	return false, nil
}

func handleHostNetworkFalseWithHostPorts(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	snaps, err := sdkobjectpatch.UnmarshalToStruct[bool](input.Snapshots, "daemonset")
	if err != nil {
		return fmt.Errorf("failed to unmarshal daemonset snapshot: %w", err)
	}

	if len(snaps) == 0 {
		return nil
	}

	if !snaps[0] {
		return nil
	}

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("can't init Kubernetes client: %v", err)
	}

	ds, err := k8sClient.AppsV1().DaemonSets("d8-istio").Get(context.TODO(), "istio-cni-node", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get latest version of istio-cni-node DaemonSet: %v", err)
	}

	for containerIdx := range ds.Spec.Template.Spec.Containers {
		for portIdx := range ds.Spec.Template.Spec.Containers[containerIdx].Ports {
			ds.Spec.Template.Spec.Containers[containerIdx].Ports[portIdx].HostPort = 0
		}
	}

	_, err = k8sClient.AppsV1().DaemonSets("d8-istio").Update(context.TODO(), ds, metav1.UpdateOptions{})

	return err
}
