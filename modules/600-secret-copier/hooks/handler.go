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
	"log/slog"
	"reflect"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

const (
	secretCopierEnableKey            = "secret-copier.deckhouse.io/enabled"
	secretCopierNamespaceSelectorKey = "secret-copier.deckhouse.io/target-namespace-selector"
)

// protectedSecrets are never created, updated or deleted by the hook, whatever
// the Secrets in `default` say. The hook runs as the deckhouse service account,
// which the admission policies guarding these Secrets trust, so anybody who can
// create a Secret in `default` could otherwise plant, replace or delete them:
// d8-system/cluster-key holds the cluster key of licensing, its identity.
var protectedSecrets = map[string]bool{
	"d8-system/cluster-key": true,
}

// isCopyable tells a Secret in `default` the hook copies. A bootstrap token is
// never copied: kube-apiserver accepts one found in kube-system as credentials
// (--enable-bootstrap-token-auth), and the hook would plant it there for anybody
// who can create a Secret in `default`. Its copies made before are deleted as
// no longer desired.
func isCopyable(secret Secret) bool {
	return secret.Type != v1.SecretTypeBootstrapToken
}

// isCopy tells a copy the hook made from a Secret of the same name that is
// somebody else's: every copy carries the label of its source. The hook updates
// and deletes copies only.
func isCopy(secret *v1.Secret) bool {
	_, ok := secret.Labels[secretCopierEnableKey]
	return ok
}

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

func SecretPath(s Secret) string {
	return fmt.Sprintf("%s/%s", s.Namespace, s.Name)
}

func ApplyCopierSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	s := Secret{
		Name:        secret.Name,
		Namespace:   secret.Namespace,
		Annotations: secret.Annotations,
		Labels:      secret.Labels,
		Type:        secret.Type,
		Data:        secret.Data,
	}

	if secret.Namespace != v1.NamespaceDefault {
		// desired secret (target secret in a namespace) must not have annotations to satisfy DeepEqual function
		s.Annotations = nil
	}

	// Secrets with that label lead to D8CertmanagerOrphanSecretsChecksFailed alerts.
	delete(s.Labels, "certmanager.k8s.io/certificate-name")

	// Secrets with that labels lead to ArgoCD control over them.
	delete(s.Labels, "argocd.argoproj.io/instance")
	delete(s.Labels, "argocd.argoproj.io/secret-type")

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

func copierHandler(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	secrets, err := sdkobjectpatch.UnmarshalToStruct[Secret](input.Snapshots, "secrets")
	if err != nil {
		return fmt.Errorf("failed to unmarshal secrets snapshot: %w", err)
	}
	if len(secrets) == 0 {
		input.Logger.Info("No Secrets received, skipping execution")
		return nil
	}

	namespaces, err := sdkobjectpatch.UnmarshalToStruct[Namespace](input.Snapshots, "namespaces")
	if err != nil {
		return fmt.Errorf("failed to unmarshal namespaces snapshot: %w", err)
	}
	if len(namespaces) == 0 {
		input.Logger.Info("No Namespaces received, skipping execution")
		return nil
	}

	k8, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("can't init Kubernetes client: %w", err)
	}

	secretsExists := make(map[string]Secret)
	secretsDesired := make(map[string]Secret)

	for _, secret := range secrets {
		// Secrets that are not in namespace `default` are existing Secrets.
		if secret.Namespace != v1.NamespaceDefault {
			path := SecretPath(secret)
			secretsExists[path] = secret
			continue
		}
		if !isCopyable(secret) {
			continue
		}

		namespaceLabelSelector := namespaceSelector(secret)

		// Secrets in namespace `default` should be propagated to all other namespaces matching the selector.
		for _, namespace := range namespaces {
			if namespace.IsTerminating || namespace.Name == v1.NamespaceDefault {
				continue
			}
			namespaceLabels := labels.Set(namespace.Labels)
			if !namespaceLabelSelector.Matches(namespaceLabels) {
				continue
			}
			secretDesired := Secret{
				Name:      secret.Name,
				Namespace: namespace.Name,
				Labels:    secret.Labels,
				Type:      secret.Type,
				Data:      secret.Data,
			}
			path := SecretPath(secretDesired)
			if protectedSecrets[path] {
				continue
			}
			secretsDesired[path] = secretDesired
		}
	}

	for path, secretExist := range secretsExists {
		if protectedSecrets[path] {
			continue
		}
		secretDesired, desired := secretsDesired[path]
		if !desired {
			// Secret exists, but not desired - delete it.
			if err := deleteSecret(k8, input.Logger, secretExist); err != nil {
				return err
			}
			continue
		}
		if !reflect.DeepEqual(secretDesired, secretExist) {
			// Secret changed - update it.
			if err := createOrUpdateSecret(k8, input.Logger, secretDesired); err != nil {
				return err
			}
		}
	}
	for path, secretDesired := range secretsDesired {
		if _, exists := secretsExists[path]; exists {
			continue
		}
		// Secret not exists, create it.
		if err := createOrUpdateSecret(k8, input.Logger, secretDesired); err != nil {
			return err
		}
	}

	return nil
}

// todo(31337Ghost) consider switching to separate create/update functions after a bug is fixed in shell-operator that causes missing Secrets in snapshots
func createOrUpdateSecret(k8 k8s.Client, logger go_hook.Logger, secret Secret) error {
	existing, err := k8.CoreV1().Secrets(secret.Namespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return createSecret(k8, secret)
	} else if err != nil {
		return err
	}
	if !isCopy(existing) {
		logSkippedSecret(logger, secret)
		return nil
	}

	return updateSecret(k8, existing, secret)
}

func createSecret(k8 k8s.Client, secret Secret) error {
	s := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name,
			Namespace: secret.Namespace,
			Labels:    secret.Labels,
			Annotations: map[string]string{
				"secret-copier.deckhouse.io/created-at": time.Now().UTC().Format(time.RFC3339),
			},
		},
		Data: secret.Data,
		Type: secret.Type,
	}

	// Ignore error if target namespace is in Terminating phase
	if _, err := k8.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), s, metav1.CreateOptions{}); err != nil {
		if !errors.HasStatusCause(err, v1.NamespaceTerminatingCause) {
			return formatSecretOperationError(secret, err, "create")
		}
	}

	return nil
}

// deleteSecret deletes a copy that is no longer desired. The snapshot may lag
// behind, so the Secret is read again: one that took the place of the copy
// since is not the hook's to delete.
func deleteSecret(k8 k8s.Client, logger go_hook.Logger, secret Secret) error {
	existing, err := k8.CoreV1().Secrets(secret.Namespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return formatSecretOperationError(secret, err, "get")
	}
	if !isCopy(existing) {
		logSkippedSecret(logger, secret)
		return nil
	}

	if err := deleteCopy(k8, existing); err != nil {
		return formatSecretOperationError(secret, err, "delete")
	}

	return nil
}

// deleteCopy deletes the very Secret that was checked to be a copy: the
// preconditions fail the request if another one took its place since.
func deleteCopy(k8 k8s.Client, existing *v1.Secret) error {
	return k8.CoreV1().Secrets(existing.Namespace).Delete(context.TODO(), existing.Name, metav1.DeleteOptions{
		Preconditions: &metav1.Preconditions{UID: &existing.UID, ResourceVersion: &existing.ResourceVersion},
	})
}

func logSkippedSecret(logger go_hook.Logger, secret Secret) {
	logger.Warn("Leave the Secret as is: it has the name of a copy, but secret-copier did not create it", slog.String("secret", SecretPath(secret)))
}

// updateSecret writes the desired content over existing, the copy that was
// read. Pinned to its resource version, neither the update nor the delete of a
// recreate touches a Secret that took the place of the copy since.
func updateSecret(k8 k8s.Client, existing *v1.Secret, secret Secret) error {
	s := v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            secret.Name,
			Namespace:       secret.Namespace,
			ResourceVersion: existing.ResourceVersion,
			Labels:          secret.Labels,
			Annotations: map[string]string{
				"secret-copier.deckhouse.io/updated-at": time.Now().UTC().Format(time.RFC3339),
			},
		},
		Data: secret.Data,
		Type: secret.Type,
	}

	if _, err := k8.CoreV1().Secrets(secret.Namespace).Update(context.TODO(), &s, metav1.UpdateOptions{}); err != nil {
		// deleting and create Secret if its validation fails
		// usually means that we are trying to change an immutable field
		if errors.IsInvalid(err) {
			err := deleteCopy(k8, existing)
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

func formatSecretOperationError(secret Secret, err error, op string) error {
	return fmt.Errorf("can't %s secret object `%s/%s`: %v", op, secret.Namespace, secret.Name, err)
}

func namespaceSelector(secret Secret) labels.Selector {
	v, found := secret.Annotations[secretCopierNamespaceSelectorKey]
	if !found {
		return labels.Everything()
	}
	if s, err := labels.Parse(v); err == nil {
		return s
	}
	return labels.Nothing()
}
