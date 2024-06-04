/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import "github.com/deckhouse/deckhouse/go_lib/hooks/ensure_rbacv2"

var _ = ensure_rbacv2.RegisterHook("node-local-dns", []string{"networking", "infrastructure"}, nil)
