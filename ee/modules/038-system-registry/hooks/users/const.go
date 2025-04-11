/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package users

import (
	"fmt"
	"regexp"
)

const (
	userSecretNamePrefix = "registry-user-"
)

var (
	userNameRegex  = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])*$`)
	userNameMaxLen = 253 - len(userSecretNamePrefix)
)

func isValidUserName(name string) bool {
	if len(name) > userNameMaxLen {
		return false
	}

	return userNameRegex.MatchString(name)
}

func userSecretName(name string) string {
	return fmt.Sprintf("%s%s", userSecretNamePrefix, name)
}
