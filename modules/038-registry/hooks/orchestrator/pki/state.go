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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

type State struct {
	CA    *CertModel `json:"ca,omitempty"`
	Token *CertModel `json:"token,omitempty"`
}

type Result struct {
	CA    pki.CertKey
	Token pki.CertKey
}

func (state *State) Process(log go_hook.Logger) (Result, error) {
	var (
		ret Result
		err error
	)

	// CA
	ret.CA, err = state.CA.toPKI()
	if err != nil {
		log.Warn("Cannot decode CA certificate and key, will generate new", "error", err)

		ret.CA, err = pki.GenerateCACertificate("registry-ca")
		if err != nil {
			return ret, fmt.Errorf("cannot generate CA certificate: %w", err)
		}
	}

	state.CA, err = pkiCertModel(ret.CA)
	if err != nil {
		return ret, fmt.Errorf("cannot convert CA PKI to model: %w", err)
	}

	// Token
	ret.Token, err = state.Token.toPKI()
	if err == nil {
		err = pki.ValidateCertWithCAChain(ret.Token.Cert, ret.CA.Cert)
		if err != nil {
			log.Warn("Token certificate is not belongs to CA certificate", "error", err)
		}
	}

	if err != nil {
		ret.Token, err = pki.GenerateCertificate("registry-auth-token", ret.CA)
		if err != nil {
			return ret, fmt.Errorf("cannot generate Token certificate: %w", err)
		}
	}

	state.Token, err = pkiCertModel(ret.Token)
	if err != nil {
		return ret, fmt.Errorf("cannot convert Token PKI to model: %w", err)
	}

	return ret, nil
}
