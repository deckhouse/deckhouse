/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package users

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	snapUsers = "users"
)

var (
	namespaceSelector = &types.NamespaceSelector{
		NameSelector: &types.NameSelector{
			MatchNames: []string{"d8-system"},
		},
	}
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/system-registry/users",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              snapUsers,
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: namespaceSelector,
			NameSelector: &types.NameSelector{
				MatchNames: []string{
					"registry-user-ro",
					"registry-user-rw",
					"registry-user-mirror-puller",
					"registry-user-mirror-pusher",
				},
			},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var secret v1core.Secret

				err := sdk.FromUnstructured(obj, &secret)
				if err != nil {
					return "", fmt.Errorf("failed to convert secret \"%v\" to struct: %v", obj.GetName(), err)
				}

				return "", nil
			},
		},
	},
}, func(input *go_hook.HookInput) error {

	return nil
})
