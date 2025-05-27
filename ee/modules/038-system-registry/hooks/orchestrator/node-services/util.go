/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package nodeservices

import (
	"strings"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/users"
	nodeservices "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/node-services"
)

func mapUser(user users.User) nodeservices.User {
	return nodeservices.User{
		Name:         user.UserName,
		Password:     user.Password,
		PasswordHash: user.HashedPassword,
	}
}

func getRegistryAddressAndPathFromImagesRepo(imgRepo string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(strings.TrimRight(imgRepo, "/")), "/", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], "/" + parts[1]
}
