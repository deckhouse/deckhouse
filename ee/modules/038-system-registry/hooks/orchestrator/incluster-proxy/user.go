/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package inclusterproxy

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/users"
)

func processUserPasswordHash(log go_hook.Logger, user *users.User) error {
	log = log.With("action", "ProcessUserPasswordHash", "username", user.UserName)

	if user.IsPasswordHashValid() {
		return nil
	}
	log.Warn("Password hash is invalid")

	log.Info("Generating new password hash")
	if err := user.UpdatePasswordHash(); err != nil {
		return fmt.Errorf("failed to update password hash for user \"%s\": %w", user.UserName, err)
	}
	return nil
}
