/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/change_host_address"
)

var _ = change_host_address.RegisterHook("system-registry", "d8-system")
