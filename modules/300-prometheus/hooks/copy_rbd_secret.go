package hooks

import (
	"errors"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var executeHookEmitter = false

type StorageClassObject struct {
	UserSecretName string
}

func filterStorageClass(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	sc := new(storagev1.StorageClass)

	err := sdk.FromUnstructured(obj, sc)
	if err != nil {
		return nil, err
	}

	sco := new(StorageClassObject)
	if sc.Provisioner == "kubernetes.io/rbd" {
		userSecretName, ok := sc.Parameters["userSecretName"]
		if !ok {
			return nil, errors.New("no user secret found")
		}
		sco.UserSecretName = userSecretName
	}

	return sco, nil
}

type SecretObject v1.Secret

func filterSecrets(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	sc := new(v1.Secret)
	err := sdk.FromUnstructured(obj, sc)
	if err != nil {
		return nil, err
	}

	return SecretObject(*sc), nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "rbd_storageclass",
			ApiVersion:                   "storage.k8s.io/v1",
			Kind:                         "StorageClass",
			ExecuteHookOnSynchronization: &executeHookEmitter,
			ExecuteHookOnEvents:          &executeHookEmitter,
			FilterFunc:                   filterStorageClass,
		},
		{
			Name:                         "rbd_secret",
			ApiVersion:                   "v1",
			Kind:                         "Secret",
			ExecuteHookOnSynchronization: &executeHookEmitter,
			ExecuteHookOnEvents:          &executeHookEmitter,
			FilterFunc:                   filterSecrets,
		},
	},
}, copyRBDSecretHandler)

func copyRBDSecretHandler(input *go_hook.HookInput) error {
	storageClassSnap, ok := input.Snapshots["rbd_storageclass"]
	if !ok {
		return errors.New("no storageClasses snapshot")
	}

	secretSnap, ok := input.Snapshots["rbd_secret"]
	if !ok {
		return errors.New("no secrets snapshot")
	}

	secretsToCopy := make(map[string]SecretObject)
	d8Secrets := make(map[string]bool)

	for _, secret := range secretSnap {
		secret := secret.(SecretObject)

		if secret.Namespace == "d8-monitoring" {
			d8Secrets[secret.Name] = true
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

	for _, storageClass := range storageClassSnap {
		userSecret := storageClass.(*StorageClassObject).UserSecretName
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
