/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package users

import (
	"fmt"
)

const (
	SecretNamePrefix = "registry-user-"
)

func SecretName(name string) string {
	return fmt.Sprintf("%s%s", SecretNamePrefix, name)
}
