package hooks

import "github.com/deckhouse/deckhouse/go_lib/hooks/change_host_address"

var _ = change_host_address.RegisterHook("cloud-controller-manager", "d8-cloud-provider-azure")
