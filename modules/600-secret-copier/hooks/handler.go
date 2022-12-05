/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

const (
	secretCopierEnableKey            = "secret-copier.deckhouse.io/enabled"
	secretCopierNamespaceSelectorKey = "secret-copier.deckhouse.io/target-namespace-selector"
)

type Secret struct {
	Name        string
	Namespace   string
	Annotations map[string]string
	Labels      map[string]string
	Type        v1.SecretType     `json:"type,omitempty"`
	Data        map[string][]byte `json:"data,omitempty"`
}

type Namespace struct {
	Name          string `json:"name,omitempty"`
	Labels        map[string]string
	IsTerminating bool `json:"is_terminating,omitempty"`
}

func SecretPath(s *Secret) string {
	return fmt.Sprintf("%s/%s", s.Namespace, s.Name)
}

func ApplyCopierSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	s := &Secret{
		Name:        secret.Name,
		Namespace:   secret.Namespace,
		Annotations: secret.Annotations,
		Labels:      secret.Labels,
		Type:        secret.Type,
		Data:        secret.Data,
	}
	// Secrets with that label lead to D8CertmanagerOrphanSecretsChecksFailed alerts.
	delete(s.Labels, "certmanager.k8s.io/certificate-name")

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
		Labels:        namespace.Labels,
		IsTerminating: namespace.Status.Phase == v1.NamespaceTerminating,
	}

	return n, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 5 * time.Second,
		ExecutionBurst:       3,
	},
	Queue: "/modules/secret-copier",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secrets",
			ApiVersion: "v1",
			Kind:       "Secret",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      secretCopierEnableKey,
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc:             ApplyCopierSecretFilter,
			WaitForSynchronization: go_hook.Bool(false),
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
			FilterFunc:             ApplyCopierNamespaceFilter,
			WaitForSynchronization: go_hook.Bool(false),
		},
	},
}, dependency.WithExternalDependencies(copierHandler))

func copierHandler(input *go_hook.HookInput, dc dependency.Container) error {
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

	k8, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("can't init Kubernetes client: %v", err)
	}

	secretsExists := make(map[string]*Secret)
	secretsDesired := make(map[string]*Secret)
	for _, s := range secrets {
		secret := s.(*Secret)
		// Secrets that are not in namespace `default` are existing Secrets.
		if secret.Namespace != v1.NamespaceDefault {
			path := SecretPath(secret)
			secretsExists[path] = secret
			continue
		}
		namespaceLabelSelector := namespaceSelector(secret)

		// Secrets in namespace `default` should be propagated to all other namespaces matching the selector.
		for _, n := range namespaces {
			namespace := n.(*Namespace)
			if namespace.IsTerminating || namespace.Name == v1.NamespaceDefault {
				continue
			}
			namespaceLabels := labels.Set(namespace.Labels)
			if !namespaceLabelSelector.Matches(namespaceLabels) {
				continue
			}
			secretDesired := &Secret{
				Name:      secret.Name,
				Namespace: namespace.Name,
				Labels:    secret.Labels,
				Type:      secret.Type,
				Data:      secret.Data,
			}
			path := SecretPath(secretDesired)
			secretsDesired[path] = secretDesired
		}
	}

	for path, secretExist := range secretsExists {
		secretDesired, desired := secretsDesired[path]
		if !desired {
			// Secret exists, but not desired - delete it.
			err := deleteSecret(k8, secretExist)
			if err != nil {
				return err
			}
			continue
		}
		if !reflect.DeepEqual(secretDesired, secretExist) {
			// Secret changed - update it.
			err = createOrUpdateSecret(k8, secretDesired)
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
		err := createOrUpdateSecret(k8, secretDesired)
		if err != nil {
			return err
		}
	}

	return nil
}

// todo(31337Ghost) consider switching to separate create/update functions after a bug is fixed in shell-operator that causes missing Secrets in snapshots
func createOrUpdateSecret(k8 k8s.Client, secret *Secret) error {
	_, err := k8.CoreV1().Secrets(secret.Namespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return createSecret(k8, secret)
	} else if err != nil {
		return err
	}

	return updateSecret(k8, secret)
}

func createSecret(k8 k8s.Client, secret *Secret) error {
	s := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name,
			Namespace: secret.Namespace,
			Labels:    secret.Labels,
		},
		Data: secret.Data,
		Type: secret.Type,
	}
	if _, err := k8.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), s, metav1.CreateOptions{}); err != nil {
		return formatSecretOperationError(secret, err, "create")
	}

	return nil
}

func deleteSecret(k8 k8s.Client, secret *Secret) error {
	if err := k8.CoreV1().Secrets(secret.Namespace).Delete(context.TODO(), secret.Name, metav1.DeleteOptions{}); err != nil {
		return formatSecretOperationError(secret, err, "delete")
	}

	return nil
}

func updateSecret(k8 k8s.Client, secret *Secret) error {
	s := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name,
			Namespace: secret.Namespace,
			Labels:    secret.Labels,
		},
		Data: secret.Data,
		Type: secret.Type,
	}

	if _, err := k8.CoreV1().Secrets(secret.Namespace).Update(context.TODO(), s, metav1.UpdateOptions{}); err != nil {
		// deleting and create Secret if its validation fails
		// usually means that we are trying to change an immutable field
		if errors.IsInvalid(err) {
			err := deleteSecret(k8, secret)
			if err != nil {
				return formatSecretOperationError(secret, err, "delete on recreate")
			}
			err = createSecret(k8, secret)
			if err != nil {
				return formatSecretOperationError(secret, err, "create after delete on recreate")
			}
			return nil
		}
		return formatSecretOperationError(secret, err, "update")
	}

	return nil
}

func formatSecretOperationError(secret *Secret, err error, op string) error {
	return fmt.Errorf("can't %s secret object `%s/%s`: %v", op, secret.Namespace, secret.Name, err)
}

func namespaceSelector(secret *Secret) labels.Selector {
	v, found := secret.Annotations[secretCopierNamespaceSelectorKey]
	if !found {
		return labels.Everything()
	}
	if s, err := labels.Parse(v); err == nil {
		return s
	}
	return labels.Nothing()
}
