/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package inclusterproxy

import (
	"strings"
)

func getRegistryAddressAndPathFromImagesRepo(imgRepo string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(strings.TrimRight(imgRepo, "/")), "/", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], "/" + parts[1]
}
