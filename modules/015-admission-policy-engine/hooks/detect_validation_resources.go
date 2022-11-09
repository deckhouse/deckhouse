package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "constraints",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-admission-policy-engine"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"gatekeeper-controller-manager"},
			},
			FilterFunc: filterGatekeeperDeployment,
		},
	},
}, dependency.WithExternalDependencies(xxxx))

func xxxx(input *go_hook.HookInput, dc dependency.Container) error {
	k8, err := dc.GetK8sClient()

	k8.Dynamic().Resource(v1.GroupVersionResource{
		Group:    "constraints.gatekeeper.sh",
		Version:  "v1beta1",
		Resource: "",
	}).List()
}
