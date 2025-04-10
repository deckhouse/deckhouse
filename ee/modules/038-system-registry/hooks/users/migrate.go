/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package users

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/module-sdk/pkg/utils/ptr"
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
}, dependency.WithExternalDependencies(func(input *go_hook.HookInput, dc dependency.Container) error {
	secrets, err := helpers.SnapshotToList[v1core.Secret](input, snapMigrateSecrets)
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
		newSecret.Annotations = secret.Annotations
		newSecret.Data = secret.Data

		if newSecret.Labels == nil {
			newSecret.Labels = make(map[string]string)
		}
		newSecret.Labels["app.kubernetes.io/managed-by"] = "Helm"
		delete(newSecret.Labels, "migrate")

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
