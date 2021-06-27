/*
Copyright 2021 Flant CJSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE
*/

package hooks

import (
	"fmt"
	"reflect"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Secret struct {
	ObjectMeta metav1.ObjectMeta `json:"metadata,omitempty"`
	Type       v1.SecretType     `json:"type,omitempty"`
	Data       map[string][]byte `json:"data,omitempty"`
}

type Namespace struct {
	Name          string `json:"name,omitempty"`
	IsTerminating bool   `json:"is_terminating,omitempty"`
}

func SecretPath(s *Secret) string {
	return fmt.Sprintf("%s/%s", s.ObjectMeta.Namespace, s.ObjectMeta.Name)
}

func ApplyCopierSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	s := &Secret{
		ObjectMeta: secret.ObjectMeta,
		Type:       secret.Type,
		Data:       secret.Data,
	}
	// Secrets with that label lead to D8CertmanagerOrphanSecretsChecksFailed alerts.
	delete(s.ObjectMeta.Labels, "certmanager.k8s.io/certificate-name")

	return s, nil
}

func ApplyCopierNamespaceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	namespace := &v1.Namespace{}
	err := sdk.FromUnstructured(obj, namespace)
	if err != nil {
		return nil, err
	}

	n := &Namespace{
		Name:          namespace.ObjectMeta.Name,
		IsTerminating: namespace.Status.Phase == v1.NamespaceTerminating,
	}

	return n, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secrets",
			ApiVersion: "v1",
			Kind:       "Secret",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"secret-copier.deckhouse.io/enabled": "",
				},
			},
			FilterFunc: ApplyCopierSecretFilter,
		},
		{
			Name:       "namespaces",
			ApiVersion: "v1",
			Kind:       "Namespace",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpNotIn,
						Values: []string{
							"upmeter",
						},
					},
				},
			},
			FilterFunc: ApplyCopierNamespaceFilter,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "cron",
			Crontab: "0 3 * * *",
		},
	},
}, copierHandler)

func copierHandler(input *go_hook.HookInput) error {
	secrets, ok := input.Snapshots["secrets"]
	if !ok {
		input.LogEntry.Info("No Secrets received, skipping execution")
		return nil
	}
	namespaces, ok := input.Snapshots["namespaces"]
	if !ok {
		input.LogEntry.Info("No Namespaces received, skipping execution")
		return nil
	}

	secretsExists := make(map[string]*Secret)
	secretsDesired := make(map[string]*Secret)
	for _, s := range secrets {
		secret := s.(*Secret)
		// Secrets that are not in namespace `default` are existing Secrets.
		if secret.ObjectMeta.Namespace != "default" {
			path := SecretPath(secret)
			secretsExists[path] = secret
			continue
		}
		// Secrets in namespace `default` should be propagated to all other active namespaces.
		for _, n := range namespaces {
			namespace := n.(*Namespace)
			if namespace.IsTerminating || namespace.Name == "default" {
				continue
			}
			secretDesired := &Secret{
				ObjectMeta: secret.ObjectMeta,
				Type:       secret.Type,
				Data:       secret.Data,
			}
			secretDesired.ObjectMeta.Namespace = namespace.Name
			path := SecretPath(secretDesired)
			secretsDesired[path] = secretDesired
		}
	}

	for path, secretExist := range secretsExists {
		secretDesired, desired := secretsDesired[path]
		if !desired {
			// Secret exists, but not desired - delete it.
			_ = input.ObjectPatcher.DeleteObject("", "Secret", secretExist.ObjectMeta.Namespace, secretExist.ObjectMeta.Name, "")
			continue
		}
		if !reflect.DeepEqual(secretDesired, secretExist) {
			// Secret changed - update it.
			err := CreateOrUpdateSecret(input, secretDesired)
			if err != nil {
				return err
			}
		}
	}
	for path, secretDesired := range secretsDesired {
		_, exists := secretsExists[path]
		if exists {
			continue
		}
		// Secret not exists, create it.
		err := CreateOrUpdateSecret(input, secretDesired)
		if err != nil {
			return err
		}
	}

	return nil
}

func CreateOrUpdateSecret(input *go_hook.HookInput, secret *Secret) error {
	s := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.ObjectMeta.Name,
			Namespace: secret.ObjectMeta.Namespace,
			Labels:    secret.ObjectMeta.Labels,
		},
		Data: secret.Data,
		Type: secret.Type,
	}
	su, err := sdk.ToUnstructured(s)
	if err != nil {
		return fmt.Errorf("can't convert Secret to Unstructured: %v", err)
	}
	if err := input.ObjectPatcher.CreateOrUpdateObject(su, ""); err != nil {
		return fmt.Errorf("can't CreateOrUpdateObject Secret object: %v", err)
	}

	return nil
}
