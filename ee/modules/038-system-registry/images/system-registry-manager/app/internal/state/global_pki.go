/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import (
	"embeded-registry-manager/internal/utils/pki"
)

const (
	PKISecretName = "registry-pki"

	CASecretType      = "system-registry/ca-secret"
	CASecretTypeLabel = "global-pki-secret"

	CACertSecretField = "registry-ca.crt"
	CAKeySecretField  = "registry-ca.key"

	TokenCertSecretField = "token.crt"
	TokenKeySecretField  = "token.key"
)

type GlobalPKI struct {
	CA    *pki.CertKey
	Token *pki.CertKey
}
