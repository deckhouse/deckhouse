// Copyright 2026 Flant JSC
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

package registrymocks

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func CreatePKISecret(ctx context.Context, kubeClient client.KubeClient) error {
	pki, err := registry.GeneratePKI()
	if err != nil {
		return err
	}

	pkiYAML, err := yaml.Marshal(pki)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-init",
			Namespace: "d8-system",
		},
		Data: map[string][]byte{
			"config": pkiYAML,
		},
	}

	return createOrUpdateSecret(ctx, kubeClient, secret)
}

func createOrUpdateSecret(ctx context.Context, kubeClient client.KubeClient, secret *corev1.Secret) error {
	_, err := kubeClient.
		CoreV1().
		Secrets(secret.Namespace).
		Create(ctx, secret, metav1.CreateOptions{})

	if err != nil && apierrors.IsAlreadyExists(err) {
		_, err = kubeClient.
			CoreV1().
			Secrets(secret.Namespace).
			Update(ctx, secret, metav1.UpdateOptions{})
	}
	return err
}
