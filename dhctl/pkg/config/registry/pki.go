// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

const (
	certificateCommonName = "registry-ca"
	roUsername            = "ro"
	rwUsername            = "rw"
)

func GetPKI(ctx context.Context, kubeClient client.KubeClient) (PKI, error) {
	secret, err := kubeClient.
		CoreV1().
		Secrets(secretsNamespace).
		Get(ctx, initSecretName, metav1.GetOptions{})
	if err != nil {
		return PKI{}, fmt.Errorf("get secret '%s/%s': %w", secretsNamespace, initSecretName, err)
	}

	var ret PKI
	if err := yaml.Unmarshal(secret.Data["config"], &ret); err != nil {
		return PKI{}, fmt.Errorf("unmarshal secret data: %w", err)
	}

	return ret, nil
}

// EnsureInitSecret uploads the registry PKI to `d8-system/registry-init` on a cluster whose first
// master runs no bashible, and leaves a secret that is already there as it is.
//
// On every other cluster this is the job of the bootstrap step 073_init_registry_secrets: the node
// uploads the PKI dhctl handed it in the bashible bundle, and GetPKI reads it back to build the
// `deckhouse-registry` secret. An immutable master runs no such step, so without this the
// installation stops at "get PKI: secrets \"registry-init\" not found" with the control plane up.
//
// The secret is the same one that step writes — same name, same `config` key, and the
// `is-applied` annotation on a bundle bootstrap for the reason the step gives — so its readers
// cannot tell which path wrote it. An existing secret is kept rather than regenerated, because on
// a repeated run it is the PKI the cluster may already be serving.
func EnsureInitSecret(ctx context.Context, kubeClient client.KubeClient, fromBundle bool) error {
	return retry.
		NewLoop(fmt.Sprintf("Upload the registry PKI to %s/%s", secretsNamespace, initSecretName), 30, 5*time.Second).
		RunContext(ctx, func() error {
			return ensureInitSecret(ctx, kubeClient, fromBundle)
		})
}

func ensureInitSecret(ctx context.Context, kubeClient client.KubeClient, fromBundle bool) error {
	secrets := kubeClient.CoreV1().Secrets(secretsNamespace)

	_, err := secrets.Get(ctx, initSecretName, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get secret '%s/%s': %w", secretsNamespace, initSecretName, err)
	}

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: secretsNamespace}}
	_, err = kubeClient.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create namespace %q: %w", secretsNamespace, err)
	}

	pki, err := GeneratePKI()
	if err != nil {
		return fmt.Errorf("generate PKI: %w", err)
	}

	config, err := yaml.Marshal(pki)
	if err != nil {
		return fmt.Errorf("marshal PKI: %w", err)
	}

	annotations := map[string]string{}
	if fromBundle {
		annotations[initSecretAppliedAnnotation] = ""
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        initSecretName,
			Namespace:   secretsNamespace,
			Annotations: annotations,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"config": config},
	}

	_, err = secrets.Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create secret '%s/%s': %w", secretsNamespace, initSecretName, err)
	}

	return nil
}

func GeneratePKI() (PKI, error) {
	ca, err := generatePKICA(certificateCommonName)
	if err != nil {
		return PKI{}, fmt.Errorf("generate CA for common name %q: %w", certificateCommonName, err)
	}

	ro, err := generatePKIUser(roUsername)
	if err != nil {
		return PKI{}, fmt.Errorf("generate user %q: %w", roUsername, err)
	}

	rw, err := generatePKIUser(rwUsername)
	if err != nil {
		return PKI{}, fmt.Errorf("generate user %q: %w", rwUsername, err)
	}

	return PKI{
		CA:     ca,
		ROUser: ro,
		RWUser: rw,
	}, nil
}

func generatePKIUser(name string) (PKIUser, error) {
	password, err := pki.GenerateUserPassword()
	if err != nil {
		return PKIUser{}, fmt.Errorf("generate password: %w", err)
	}

	passwordHash, err := pki.GeneratePasswordHash(password)
	if err != nil {
		return PKIUser{}, fmt.Errorf("generate password hash: %w", err)
	}

	return PKIUser{
		Name:         name,
		Password:     password,
		PasswordHash: passwordHash,
	}, nil
}

func generatePKICA(commonName string) (PKICertKey, error) {
	certKey, err := pki.GenerateCACertificate(commonName)
	if err != nil {
		return PKICertKey{}, fmt.Errorf("generate CA certificate: %w", err)
	}

	cert, key, err := pki.EncodeCertKey(certKey)
	if err != nil {
		return PKICertKey{}, fmt.Errorf("encode CA cert/key: %w", err)
	}

	return PKICertKey{
		Cert: string(cert),
		Key:  string(key),
	}, nil
}
