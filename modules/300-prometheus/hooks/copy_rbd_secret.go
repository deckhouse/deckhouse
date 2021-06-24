package hooks

import (
	"errors"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
)

type storageClassObject struct {
	UserSecretName string
}

func filterStorageClass(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sc storagev1.StorageClass

	err := sdk.FromUnstructured(obj, &sc)
	if err != nil {
		return nil, err
	}

	var userSecretName string
	if sc.Provisioner == "kubernetes.io/rbd" {
		userSecretName = sc.Parameters["userSecretName"]
		if userSecretName == "" {
			return nil, errors.New("userSecretName for rbd StorageClass not found")
		}
	}

	return storageClassObject{UserSecretName: userSecretName}, nil
}

func filterSecrets(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1.Secret
	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}

	return &secret, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "rbd_storageclass",
			ApiVersion:                   "storage.k8s.io/v1",
			Kind:                         "StorageClass",
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			FilterFunc:                   filterStorageClass,
		},
		{
			Name:                         "rbd_secret",
			ApiVersion:                   "v1",
			Kind:                         "Secret",
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			FilterFunc:                   filterSecrets,
			FieldSelector: &types.FieldSelector{
				MatchExpressions: []types.FieldSelectorRequirement{
					{
						Field:    "type",
						Operator: "Equals",
						Value:    "kubernetes.io/rbd",
					},
				},
			},
		},
	},
}, copyRBDSecretHandler)

func copyRBDSecretHandler(input *go_hook.HookInput) error {
	secretSnap := input.Snapshots["rbd_secret"]
	if len(secretSnap) == 0 {
		return nil
	}

	secretsToCopy := make(map[string]*v1.Secret)
	d8Secrets := make(map[string]struct{})

	for _, secret := range secretSnap {
		secret := secret.(*v1.Secret)

		if secret.Namespace == "d8-monitoring" {
			d8Secrets[secret.Name] = struct{}{}
			continue
		}
		v, ok := secretsToCopy[secret.Name]
		if !ok {
			secretsToCopy[secret.Name] = secret
			continue
		}

		// store latest secret in map
		if secret.CreationTimestamp.After(v.CreationTimestamp.Time) {
			secretsToCopy[secret.Name] = secret
		}
	}

	storageClassSnap := input.Snapshots["rbd_storageclass"]

	for _, storageClass := range storageClassSnap {
		userSecret := storageClass.(storageClassObject).UserSecretName
		if userSecret == "" {
			continue // non-rbd StorageClass
		}
		if _, ok := d8Secrets[userSecret]; ok {
			continue
		}

		existingSecret, ok := secretsToCopy[userSecret]
		if !ok {
			input.LogEntry.WithField("secretName", userSecret).Warn("secret not found")
			continue
		}

		newSecret := &v1.Secret{
			Data:     existingSecret.Data,
			Type:     existingSecret.Type,
			TypeMeta: existingSecret.TypeMeta,
			ObjectMeta: v12.ObjectMeta{
				Name:      existingSecret.Name,
				Namespace: "d8-monitoring",
				Labels:    existingSecret.Labels,
			},
		}

		unst, err := runtime.DefaultUnstructuredConverter.ToUnstructured(newSecret)
		if err != nil {
			return err
		}
		err = input.ObjectPatcher.CreateObject(
			&unstructured.Unstructured{Object: unst},
			"",
		)
		if err != nil {
			return err
		}
	}

	return nil
}
