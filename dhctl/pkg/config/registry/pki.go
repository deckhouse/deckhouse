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
	registry_users "github.com/deckhouse/deckhouse/go_lib/registry/models/users"
	registry_pki "github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

const (
	roUser       = "ro"
	rwUser       = "rw"
	CACommonName = "registry-ca"
)

var (
	pkiGenerator = &PKIGenerator{}
)

type PKIK8SProvider struct {
	pki *PKI
}

type PKIGenerator struct {
	pki *PKI
}

type PKI struct {
	CA     CertKey             `json:"ca" yaml:"ca"`
	UserRW registry_users.User `json:"userRW" yaml:"userRW"`
	UserRO registry_users.User `json:"userRO" yaml:"userRO"`
}

type CertKey struct {
	Cert string `json:"cert" yaml:"cert"`
	Key  string `json:"key" yaml:"key"`
}

func NewPKIGenerator() *PKIGenerator {
	return pkiGenerator
}

func NewPKIK8SProvider() *PKIGenerator {
	return pkiGenerator
}

func (provider *PKIGenerator) Get() (PKI, error) {
	if provider.pki == nil {
		pki := PKI{}
		if err := pki.process(); err != nil {
			return pki, err
		}
		provider.pki = &pki
	}
	// TODO: deepcopy
	return *provider.pki, nil
}

func (provider *PKIK8SProvider) Get() (PKI, error) {
	// TODO: get from cluster
	return NewPKIGenerator().Get()
}

func (pki *PKI) process() error {
	certKey, err := registry_pki.GenerateCACertificate(CACommonName)
	if err != nil {
		return err
	}
	cert, key, err := registry_pki.EncodeCertKey(certKey)
	if err != nil {
		return err
	}
	rw, err := registry_users.New(rwUser)
	if err != nil {
		return err
	}
	ro, err := registry_users.New(roUser)
	if err != nil {
		return err
	}
	pki.CA = CertKey{Cert: string(cert), Key: string(key)}
	pki.UserRW = rw
	pki.UserRO = ro
	return nil
}
