package hooks

import "github.com/deckhouse/deckhouse/go_lib/hooks/get_cni_secret"

var _ = get_cni_secret.RegisterHook("cloudProviderDvp")
