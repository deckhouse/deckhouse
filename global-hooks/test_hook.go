package hooks

import (
	"fmt"
	"time"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/test/read-pods",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "pods",
			ApiVersion: "v1",
			Kind:       "Pod",
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				pod := v1.Pod{}
				err := sdk.FromUnstructured(obj, &pod)
				if err != nil {
					return nil, err
				}
				return pod, nil
			},
		},
	},
}, func(input *go_hook.HookInput) error {
	pods, err := sdkobjectpatch.UnmarshalToStruct[v1.Pod](input.NewSnapshots, "pods")
	if err != nil {
		return fmt.Errorf("failed to unmarshal pods: %w", err)
	}
	fmt.Printf("[test_hook] Pods in cluster: %d\n", len(pods))

	for _, pod := range pods {
		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					"test-hook": "true",
				},
			},
		}
		fmt.Printf("[test_hook] Patching pod %s in namespace %s\n", pod.Name, pod.Namespace)
		input.PatchCollector.PatchWithMerge(patch, "v1", "Pod", pod.Namespace, pod.Name)
	}

	fmt.Printf("[test_hook] Patched %d pods\n", len(pods))

	fmt.Printf("[test_hook] Sleeping for 60 seconds\n")
	time.Sleep(60 * time.Second)
	fmt.Printf("[test_hook] Waking up\n")
	return nil
})
