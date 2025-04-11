package hooks

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(removeDeckhouseEpslices))

func removeDeckhouseEpslices(_ *go_hook.HookInput, dc dependency.Container) error {
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("get kubernetes client: %v", err)
	}

	return kubeClient.DiscoveryV1().EndpointSlices("d8-system").Delete(context.TODO(), "deckhouse", metav1.DeleteOptions{})
}
