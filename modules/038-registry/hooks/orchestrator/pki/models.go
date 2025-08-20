/*
Copyright 2025 Flant JSC

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

package pki

import (
	"fmt"

	v1core "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

type CertModel struct {
	Cert string `json:"cert,omitempty"`
	Key  string `json:"key,omitempty"`
}

func (pcm *CertModel) toPKI() (pki.CertKey, error) {
	if pcm == nil {
		return pki.CertKey{}, fmt.Errorf("cannot convert nil to CertKey")
	}
	return pki.DecodeCertKey([]byte(pcm.Cert), []byte(pcm.Key))
}

func pkiCertModel(value pki.CertKey) (*CertModel, error) {
	cert, key, err := pki.EncodeCertKey(value)
	if err != nil {
		return nil, err
	}
	return &CertModel{Cert: string(cert), Key: string(key)}, nil
}

func secretDataToCertModel(secret v1core.Secret, key string) *CertModel {
	if key == "" {
		return nil
	}

	certValue := string(secret.Data[fmt.Sprintf("%s.crt", key)])
	keyValue := string(secret.Data[fmt.Sprintf("%s.key", key)])

	if certValue != "" && keyValue != "" {
		return &CertModel{
			Cert: certValue,
			Key:  keyValue,
		}
	}

	return nil
}

type Inputs = State
