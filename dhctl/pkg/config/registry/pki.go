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
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	registry_init "github.com/deckhouse/deckhouse/go_lib/registry/models/init"
	registry_users "github.com/deckhouse/deckhouse/go_lib/registry/models/users"
	registry_pki "github.com/deckhouse/deckhouse/go_lib/registry/pki"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

const (
	readOnlyUser          = "ro"
	readWriteUser         = "rw"
	certificateCommonName = "registry-ca"
	secretNamespace       = "d8-system"
	secretName            = "registry-init"
)

type PKI = registry_init.Config

type ClusterPKIManager struct {
	kubeClient client.KubeClient
	once       once[PKI]
}

type PKIGenerator struct {
	once once[PKI]
}

type once[T any] struct {
	ret  T
	err  error
	once sync.Once
}

func (li *once[T]) do(initFunc func() (T, error)) (T, error) {
	li.once.Do(func() {
		li.ret, li.err = initFunc()
	})
	return li.ret, li.err
}

func (generator *PKIGenerator) Get() (PKI, error) {
	ret, err := generator.once.do(func() (PKI, error) {
		return generatePKI()
	})

	if err != nil {
		return PKI{}, fmt.Errorf("failed to generate registry PKI: %w", err)
	}
	return ret.DeepCopy(), nil
}

func (manager *ClusterPKIManager) Get() (PKI, error) {
	ret, err := manager.once.do(func() (PKI, error) {
		return fetchPKIFromCluster(context.TODO(), manager.kubeClient)
	})

	if err != nil {
		return PKI{}, fmt.Errorf("failed to retrieve registry PKI from cluster: %w", err)
	}
	return ret.DeepCopy(), nil
}

func NewPKIGenerator() *PKIGenerator {
	return &PKIGenerator{}
}

func NewClusterPKIManager(kubeClient client.KubeClient) *ClusterPKIManager {
	return &ClusterPKIManager{kubeClient: kubeClient}
}

func generatePKI() (PKI, error) {
	ret := PKI{}

	certKey, err := registry_pki.GenerateCACertificate(certificateCommonName)
	if err != nil {
		return ret, err
	}

	cert, key, err := registry_pki.EncodeCertKey(certKey)
	if err != nil {
		return ret, err
	}

	rw, err := registry_users.New(readWriteUser)
	if err != nil {
		return ret, err
	}

	ro, err := registry_users.New(readOnlyUser)
	if err != nil {
		return ret, err
	}

	ret.CA = &registry_init.CertKey{
		Cert: string(cert),
		Key:  string(key),
	}
	ret.UserRW = &rw
	ret.UserRO = &ro

	return ret, nil
}

func fetchPKIFromCluster(ctx context.Context, kubeClient client.KubeClient) (PKI, error) {
	var ret PKI
	secret, err := kubeClient.CoreV1().Secrets(secretNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return ret, err
	}

	if err := yaml.Unmarshal(secret.Data["config"], &ret); err != nil {
		return ret, fmt.Errorf("failed to unmarshal: %w", err)
	}
	return ret, nil
}
