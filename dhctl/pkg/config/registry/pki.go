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
	registry_pki "github.com/deckhouse/deckhouse/go_lib/registry/pki"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

const (
	certificateCommonName = "registry-ca"
)

type PKI = registry_init.Config
type CertKey = registry_init.CertKey

type ClusterPKIManager struct {
	kubeClient client.KubeClient

	mu  sync.RWMutex
	pki *PKI
}

type LazyPKIGenerator struct {
	once sync.Once
	pki  PKI
	err  error
}

func NewLazyPKIGenerator() *LazyPKIGenerator {
	return &LazyPKIGenerator{}
}

func NewClusterPKIManager(kubeClient client.KubeClient) *ClusterPKIManager {
	return &ClusterPKIManager{kubeClient: kubeClient}
}

func (g *LazyPKIGenerator) Get() (PKI, error) {
	g.once.Do(func() {
		g.pki, g.err = generatePKI()
	})
	if g.err != nil {
		return PKI{}, fmt.Errorf("generate registry PKI: %w", g.err)
	}
	return g.pki.DeepCopy(), nil
}

func (m *ClusterPKIManager) Get(ctx context.Context) (PKI, error) {
	m.mu.RLock()
	if m.pki != nil {
		pkiCopy := m.pki.DeepCopy()
		m.mu.RUnlock()
		return pkiCopy, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pki == nil {
		pki, err := fetchInitSecret(ctx, m.kubeClient)
		if err != nil {
			return PKI{}, fmt.Errorf("fetch registry PKI from cluster: %w", err)
		}
		m.pki = &pki
	}
	return m.pki.DeepCopy(), nil
}

func generatePKI() (PKI, error) {
	var ret PKI

	certKey, err := registry_pki.GenerateCACertificate(certificateCommonName)
	if err != nil {
		return PKI{}, fmt.Errorf("generate CA certificate: %w", err)
	}

	cert, key, err := registry_pki.EncodeCertKey(certKey)
	if err != nil {
		return PKI{}, fmt.Errorf("encode CA cert/key: %w", err)
	}

	ret.CA = &CertKey{
		Cert: string(cert),
		Key:  string(key),
	}
	return ret, nil
}
