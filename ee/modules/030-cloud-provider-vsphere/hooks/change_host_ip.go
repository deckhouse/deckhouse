/*
Copyright 2021 Flant CJSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE
*/

package hooks

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/change_host_address"
)

var _ = change_host_address.RegisterHook("cloud-controller-manager", "d8-cloud-provider-vsphere")
