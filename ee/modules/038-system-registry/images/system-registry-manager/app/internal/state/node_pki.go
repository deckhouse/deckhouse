/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import (
	"embeded-registry-manager/internal/utils/pki"
	"fmt"
	"regexp"
)

const (
	NodePKISecretType      = "system-registry/node-pki-secret"
	NodePKISecretTypeLabel = "node-pki-secret"

	NodeAuthCertCN          = "embedded-registry-auth"
	NodeAuthCertSecretField = "auth.crt"
	NodeAuthKeySecretField  = "auth.key"

	NodeDistributionCertCN          = "embedded-registry-distribution"
	NodeDistributionCertSecretField = "distribution.crt"
	NodeDistributionKeySecretField  = "distribution.key"
)

var (
	NodePKISecretRegex = regexp.MustCompile(`^registry-node-(.*)-pki$`)
)

func NodePKISecretName(nodeName string) string {
	return fmt.Sprintf("registry-node-%s-pki", nodeName)
}

type NodePKI struct {
	Auth         *pki.CertKey
	Distribution *pki.CertKey
}
