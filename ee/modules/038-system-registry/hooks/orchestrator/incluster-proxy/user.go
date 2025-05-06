/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package inclusterproxy

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/users"
)

func processUserPasswordHash(log go_hook.Logger, user users.User) (users.User, error) {
	if !user.IsPasswordHashValid() {
		log.Warn("Password hash is invalid, generating a new one.", "user", user.UserName)
		if err := user.UpdatePasswordHash(); err != nil {
			return user, fmt.Errorf("cannot update password hash for user \"%v\": %w", user.UserName, err)
		}
	}
	return user, nil
}
