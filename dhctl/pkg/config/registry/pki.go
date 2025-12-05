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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	init_secret "github.com/deckhouse/deckhouse/go_lib/registry/models/init-secret"
	"github.com/deckhouse/deckhouse/go_lib/registry/pki"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

const (
	certificateCommonName = "registry-ca"
)

type PKI = init_secret.Config
type CertKey = init_secret.CertKey

func GetPKI(ctx context.Context, kubeClient client.KubeClient) (PKI, error) {
	secret, err := kubeClient.CoreV1().Secrets(secretsNamespace).Get(ctx, initSecretName, metav1.GetOptions{})
	if err != nil {
		return PKI{}, fmt.Errorf("get secret '%s/%s': %w", secretsNamespace, initSecretName, err)
	}
	var ret PKI
	if err := yaml.Unmarshal(secret.Data["config"], &ret); err != nil {
		return PKI{}, fmt.Errorf("unmarshal secret data: %w", err)
	}
	return ret, nil
}

func GeneratePKI() (PKI, error) {
	var ret PKI

	certKey, err := pki.GenerateCACertificate(certificateCommonName)
	if err != nil {
		return PKI{}, fmt.Errorf("generate CA certificate: %w", err)
	}

	cert, key, err := pki.EncodeCertKey(certKey)
	if err != nil {
		return PKI{}, fmt.Errorf("encode CA cert/key: %w", err)
	}

	ret.CA = &CertKey{
		Cert: string(cert),
		Key:  string(key),
	}
	return ret, nil
}
