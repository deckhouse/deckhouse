/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

/*
The problem described here https://github.com/kubernetes/kubernetes/issues/117689
We can remove this hook after the minimum supported Kubernetes version will be 1.28.
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

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:       "/modules/node-local-dns",
	OnAfterHelm: &go_hook.OrderedConfig{},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "daemonset",
			ApiVersion: "apps/v1",
			Kind:       "DaemonSet",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"node-local-dns"},
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

func handleHostNetworkFalseWithHostPorts(input *go_hook.HookInput, dc dependency.Container) error {
	snap, ok := input.Snapshots["daemonset"]

	if !ok {
		return nil
	}

	if len(snap) == 0 {
		return nil
	}

	if !snap[0].(bool) {
		return nil
	}

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("can't init Kubernetes client: %v", err)
	}

	ds, err := k8sClient.AppsV1().DaemonSets("kube-system").Get(context.TODO(), "node-local-dns", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get latest version of node-local-dns DaemonSet: %v", err)
	}

	for containerIdx := range ds.Spec.Template.Spec.Containers {
		for portIdx := range ds.Spec.Template.Spec.Containers[containerIdx].Ports {
			ds.Spec.Template.Spec.Containers[containerIdx].Ports[portIdx].HostPort = 0
		}
	}

	_, err = k8sClient.AppsV1().DaemonSets("kube-system").Update(context.TODO(), ds, metav1.UpdateOptions{})

	return err
}
