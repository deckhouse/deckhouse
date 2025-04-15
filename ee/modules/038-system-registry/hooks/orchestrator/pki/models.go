/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pki

import (
	"fmt"

	v1core "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
)

type CertModel struct {
	Cert string `json:"cert,omitempty"`
	Key  string `json:"key,omitempty"`
}

func (pcm *CertModel) ToPKI() (pki.CertKey, error) {
	if pcm == nil {
		return pki.CertKey{}, fmt.Errorf("cannot convert nil to CertKey")
	}
	return pki.DecodeCertKey([]byte(pcm.Cert), []byte(pcm.Key))
}

func PKICertModel(value pki.CertKey) (*CertModel, error) {
	cert, key, err := pki.EncodeCertKey(value)
	if err != nil {
		return nil, err
	}
	return &CertModel{Cert: string(cert), Key: string(key)}, nil
}

func SecretDataToCertModel(secret v1core.Secret, key string) *CertModel {
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
