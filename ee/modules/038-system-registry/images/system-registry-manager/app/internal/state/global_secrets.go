/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import "strings"

const (
	GlobalSecretsName = "registry-secrets"

	GlobalSecretsType      = "system-registry/global-secrets"
	GlobalSecretsTypeLabel = "system-registry-global-secrets"
)

type GlobalSecrets struct {
	HttpSecret string
}

func (gs *GlobalSecrets) IsValid() bool {
	if gs == nil {
		return false
	}

	return strings.TrimSpace(gs.HttpSecret) != ""
}
