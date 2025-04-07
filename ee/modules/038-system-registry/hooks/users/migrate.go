/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package users

import (
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	migrateLabel      = "migrate"
	migrateLabelValue = "yes"

	snapMigrateSecrets = "migrate-secrets"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 2},
	Queue:        "/modules/system-registry/users-migrate",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              snapMigrateSecrets,
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
					return nil, fmt.Errorf("failed to convert secret \"%v\" to struct: %v", obj.GetName(), err)
				}

				if !strings.HasPrefix(secret.Name, userSecretNamePrefix) {
					return nil, nil
				}

				if secret.Labels[migrateLabel] == migrateLabelValue {
					return nil, nil
				}

				return secret.Name, nil
			},
		},
	},
}, func(input *go_hook.HookInput) error {
	for _, secretSnap := range input.Snapshots[snapMigrateSecrets] {
		name, ok := secretSnap.(string)
		if !ok || name == "" {
			continue
		}

		input.Logger.Warn("Migrate", "name", name)

		input.PatchCollector.PatchWithMutatingFunc(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			ret := obj.DeepCopy()

			labels := ret.GetLabels()
			if labels == nil {
				labels = make(map[string]string)
			}

			labels[migrateLabel] = migrateLabelValue

			ret.SetLabels(labels)
			return ret, nil
		}, "v1", "Secret", namespaceName, name)
	}

	return nil
})
