package change_host_address

import (
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const initialHostAddressAnnotation = "node.deckhouse.io/initial-host-ip"

type address struct {
	Name        string
	Host        string
	InitialHost string
}

func getAddress(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pod := &v1.Pod{}
	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, fmt.Errorf("cannot convert pod: %v", err)
	}

	return address{
		Name:        pod.Name,
		Host:        pod.Status.HostIP,
		InitialHost: pod.Annotations[initialHostAddressAnnotation],
	}, nil
}

func RegisterHook(name, namespace string) bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "pod",
				ApiVersion: "v1",
				Kind:       "Pod",
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{namespace},
					},
				},
				NameSelector: &types.NameSelector{
					MatchNames: []string{name},
				},
				FilterFunc: getAddress,
			},
		},
	}, wrapChangeAddressHandler(namespace))
}

func wrapChangeAddressHandler(namespace string) func(input *go_hook.HookInput) error {
	return func(input *go_hook.HookInput) error {
		return changeHostAddressHandler(namespace, input)
	}
}

func changeHostAddressHandler(namespace string, input *go_hook.HookInput) error {
	pods := input.Snapshots["pod"]
	if len(pods) == 0 {
		return nil
	}

	for _, pod := range pods {
		podAddress := pod.(address)

		if podAddress.Host == "" {
			// Pod doesn't exist, we can skip it
			continue
		}

		if podAddress.InitialHost == "" {
			jsonMergePatch, err := initialHostPatch(podAddress.Host)
			if err != nil {
				return fmt.Errorf("cannot convert patch to json: %v", err)
			}

			err = input.ObjectPatcher.MergePatchObject(
				/*patch*/ jsonMergePatch,
				/*apiVersion*/ "v1",
				/*kind*/ "Pod",
				/*namespace*/ namespace,
				/*name*/ podAddress.Name,
				/*subresource*/ "",
			)
			if err != nil {
				return fmt.Errorf("cannot patch pod %s/%s", namespace, podAddress.Name)
			}
			continue
		}

		if podAddress.InitialHost != podAddress.Host {
			err := input.ObjectPatcher.DeleteObject(
				/*apiVersion*/ "v1",
				/*kind*/ "Pod",
				/*namespace*/ namespace,
				/*name*/ podAddress.Name,
				/*subresource*/ "",
			)
			if err != nil {
				return fmt.Errorf("cannot delete pod %s/%s", namespace, podAddress.Name)
			}
		}
	}
	return nil
}

func initialHostPatch(host string) ([]byte, error) {
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				initialHostAddressAnnotation: host,
			},
		},
	}

	return json.Marshal(patch)
}
