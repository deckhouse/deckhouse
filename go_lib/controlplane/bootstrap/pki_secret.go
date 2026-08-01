/*
Copyright 2026 Flant JSC

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

package bootstrap

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki/signature"
)

const (
	pkiSecretName      = "d8-pki"
	pkiSecretNamespace = "kube-system"

	signaturePrivateKey = "signature-private"
	signaturePublicKey  = "signature-public"
)

// requiredPKIFiles maps a d8-pki secret key to its path relative to the PKI directory.
var requiredPKIFiles = map[string]string{
	"ca.crt":             "ca.crt",
	"ca.key":             "ca.key",
	"sa.pub":             "sa.pub",
	"sa.key":             "sa.key",
	"front-proxy-ca.crt": "front-proxy-ca.crt",
	"front-proxy-ca.key": "front-proxy-ca.key",
	"etcd-ca.crt":        filepath.Join("etcd", "ca.crt"),
	"etcd-ca.key":        filepath.Join("etcd", "ca.key"),
}

func ensurePKISecret(ctx context.Context, client kubernetes.Interface, pkiDir string) error {
	data, err := readPKIFiles(pkiDir)
	if err != nil {
		return err
	}

	secret, err := client.CoreV1().Secrets(pkiSecretNamespace).Get(ctx, pkiSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		newSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: pkiSecretName, Namespace: pkiSecretNamespace},
			Type:       corev1.SecretTypeOpaque,
			Data:       data,
		}
		if _, err := client.CoreV1().Secrets(pkiSecretNamespace).Create(ctx, newSecret, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create secret %s/%s: %w", pkiSecretNamespace, pkiSecretName, err)
		}
		logger.Info("created pki secret", slog.String("secret", pkiSecretName), slog.Int("keys", len(data)))

		return nil
	}
	if err != nil {
		return fmt.Errorf("get secret %s/%s: %w", pkiSecretNamespace, pkiSecretName, err)
	}

	// Only the keys built from disk are touched, the rest of the secret (for example signature keys
	// rotated by control-plane-manager) is left as is.
	changed := false
	if secret.Data == nil {
		secret.Data = make(map[string][]byte, len(data))
	}
	for key, value := range data {
		if !bytes.Equal(secret.Data[key], value) {
			secret.Data[key] = value
			changed = true
		}
	}
	if !changed {
		logger.Info("pki secret is up to date", slog.String("secret", pkiSecretName))

		return nil
	}

	if _, err := client.CoreV1().Secrets(pkiSecretNamespace).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("update secret %s/%s: %w", pkiSecretNamespace, pkiSecretName, err)
	}
	logger.Info("updated pki secret", slog.String("secret", pkiSecretName))

	return nil
}

func readPKIFiles(pkiDir string) (map[string][]byte, error) {
	data := make(map[string][]byte, len(requiredPKIFiles)+2)

	for key, relPath := range requiredPKIFiles {
		path := filepath.Join(pkiDir, relPath)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read pki file %s: %w", path, err)
		}
		data[key] = content
	}

	signatureData, err := readSignaturePair(pkiDir)
	if err != nil {
		return nil, err
	}
	for key, value := range signatureData {
		data[key] = value
	}

	return data, nil
}

// readSignaturePair reads the optional signature key pair: both files or none, one alone is an error.
func readSignaturePair(pkiDir string) (map[string][]byte, error) {
	privatePath := filepath.Join(pkiDir, signature.SignaturePrivateJWK)
	publicPath := filepath.Join(pkiDir, signature.SignaturePublicJWKS)

	privateContent, privateErr := os.ReadFile(privatePath)
	if privateErr != nil && !os.IsNotExist(privateErr) {
		return nil, fmt.Errorf("read signature file %s: %w", privatePath, privateErr)
	}

	publicContent, publicErr := os.ReadFile(publicPath)
	if publicErr != nil && !os.IsNotExist(publicErr) {
		return nil, fmt.Errorf("read signature file %s: %w", publicPath, publicErr)
	}

	if privateErr != nil && publicErr != nil {
		return nil, nil
	}
	if privateErr != nil || publicErr != nil {
		return nil, fmt.Errorf("read signature files: %s and %s are required together, only one of them exists", privatePath, publicPath)
	}

	return map[string][]byte{
		signaturePrivateKey: privateContent,
		signaturePublicKey:  publicContent,
	}, nil
}
