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

package inclusterproxy

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/users"
)

func processUserPasswordHash(log go_hook.Logger, user *users.User) error {
	log = log.With("action", "ProcessUserPasswordHash", "username", user.UserName)

	switch {
	case user.Password == "":
		user.HashedPassword = ""
		return nil

	case user.IsPasswordHashValid():
		// Valid hash already present, no update needed
		return nil

	default:
		log.Warn("Password hash is invalid; generating new hash")
		if err := user.UpdatePasswordHash(); err != nil {
			return fmt.Errorf("failed to update password hash for user %q: %w", user.UserName, err)
		}
		log.Info("New password hash generated")
		return nil
	}
}
