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

	registry_init "github.com/deckhouse/deckhouse/go_lib/registry/models/init"
	registry_users "github.com/deckhouse/deckhouse/go_lib/registry/models/users"
	registry_pki "github.com/deckhouse/deckhouse/go_lib/registry/pki"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

const (
	readOnlyUser          = "ro"
	readWriteUser         = "rw"
	certificateCommonName = "registry-ca"
)

type PKI = registry_init.Config

type ClusterPKIManager struct {
	kubeClient client.KubeClient
	pki        *PKI
	mu         sync.RWMutex
}

type PKIGenerator struct {
	pki  PKI
	err  error
	once sync.Once
}

func (generator *PKIGenerator) Get() (PKI, error) {
	generator.once.Do(func() {
		generator.pki, generator.err = generatePKI()
	})
	if generator.err != nil {
		return PKI{}, fmt.Errorf("failed to generate registry PKI: %w", generator.err)
	}
	return generator.pki.DeepCopy(), nil
}

func (manager *ClusterPKIManager) Get() (PKI, error) {
	manager.mu.RLock()
	if manager.pki != nil {
		defer manager.mu.RUnlock()
		return manager.pki.DeepCopy(), nil
	}
	manager.mu.RUnlock()

	manager.mu.Lock()
	defer manager.mu.Unlock()

	if manager.pki == nil {
		pki, err := initSecretFetch(context.TODO(), manager.kubeClient)
		if err != nil {
			return PKI{}, fmt.Errorf("failed to fetch registry PKI from cluster: %w", err)
		}
		manager.pki = &pki
	}
	return manager.pki.DeepCopy(), nil
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
