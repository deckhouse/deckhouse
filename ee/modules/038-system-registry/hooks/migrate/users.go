/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package migrate

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/module-sdk/pkg/utils/ptr"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

func init() {
	const (
		name       = "users"
		secretType = "registry/user"
	)

	sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 3},
		Queue:        fmt.Sprintf("/modules/system-registry/migrate-%s", name),
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:                         name,
				ExecuteHookOnEvents:          ptr.Bool(false),
				ExecuteHookOnSynchronization: ptr.Bool(false),
				ApiVersion:                   "v1",
				Kind:                         "Secret",
				NamespaceSelector:            helpers.NamespaceSelector,
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

					if secret.Type != secretType {
						return secret, nil
					}

					if secret.Labels["app.kubernetes.io/managed-by"] != "Helm" {
						return secret, nil
					}

					if secret.Annotations["meta.helm.sh/release-name"] != "system-registry" {
						return secret, nil
					}

					if secret.Annotations["meta.helm.sh/release-namespace"] != "d8-system" {
						return secret, nil
					}

					return nil, nil
				},
			},
		},
	}, dependency.WithExternalDependencies(func(input *go_hook.HookInput, dc dependency.Container) error {
		secrets, err := helpers.SnapshotToList[v1core.Secret](input, name)
		if err != nil {
			return fmt.Errorf("cannot get secrets: %w", err)
		}

		client, err := dc.GetK8sClient()
		if err != nil {
			return fmt.Errorf("cannot get K8S client: %w", err)
		}

		ctx := context.Background()

		for _, secret := range secrets {
			k8sSecrets := client.CoreV1().Secrets(secret.Namespace)

			var newSecret v1core.Secret

			newSecret.Type = secretType
			newSecret.Name = secret.Name
			newSecret.Namespace = secret.Namespace
			newSecret.Labels = secret.Labels
			newSecret.Data = secret.Data

			if newSecret.Labels == nil {
				newSecret.Labels = make(map[string]string)
			}
			newSecret.Labels["app.kubernetes.io/managed-by"] = "Helm"
			delete(newSecret.Labels, "migrate")

			newSecret.Annotations = secret.Annotations
			if newSecret.Annotations == nil {
				newSecret.Annotations = make(map[string]string)
			}
			newSecret.Annotations["meta.helm.sh/release-name"] = "system-registry"
			newSecret.Annotations["meta.helm.sh/release-namespace"] = "d8-system"

			input.Logger.Warn("Migrate", "name", secret.Name, "new_name", newSecret.Name)

			err = k8sSecrets.Delete(ctx, secret.Name, v1.DeleteOptions{})
			if err != nil {
				input.Logger.Warn(
					"Delete secret error",
					"name", secret.Name,
					"namespace", secret.Namespace,
				)
			}

			_, err = k8sSecrets.Create(ctx, &newSecret, v1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("cannot create secret: %w", err)
			}
		}

		return nil
	}))
}
