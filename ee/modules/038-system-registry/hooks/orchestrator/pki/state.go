/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pki

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
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
