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

	"github.com/deckhouse/module-sdk/pkg/utils/ptr"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
)

const (
	snapMigrateSecrets = "migrate-secrets"
	secretType         = "registry/user"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 3},
	Queue:        "/modules/system-registry/users-migrate",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         snapMigrateSecrets,
			ExecuteHookOnEvents:          ptr.Bool(false),
			ExecuteHookOnSynchronization: ptr.Bool(false),
			ApiVersion:                   "v1",
			Kind:                         "Secret",
			NamespaceSelector:            namespaceSelector,
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
					return nil, fmt.Errorf("failed to convert secret \"%v\" to struct: %v", obj.GetName(), err)
				}

				if secret.Type == secretType {
					return nil, nil
				}

				return secret, nil
			},
		},
	},
}, func(input *go_hook.HookInput) error {

	secrets, err := helpers.SnapshotToList[v1core.Secret](input, snapMigrateSecrets)
	if err != nil {
		return fmt.Errorf("cannot get secrets: %w", err)
	}

	for _, secret := range secrets {
		var newSecret v1core.Secret

		labels := secret.Labels
		if labels == nil {
			labels = make(map[string]string)
		}
		labels["app.kubernetes.io/managed-by"] = "Helm"

		obj := map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":        secret.Name,
				"namespace":   secret.Namespace,
				"annotations": secret.Annotations,
				"labels":      labels,
			},
			"type": secretType,
			"data": secret.Data,
		}

		input.Logger.Warn("Migrate", "name", secret.Name, "new_name", newSecret.Name)

		_ = obj

		// input.PatchCollector.Delete("v1", "Secret", secret.Namespace, secret.Name)
		// input.PatchCollector.Create(obj)
	}

	return nil
})
