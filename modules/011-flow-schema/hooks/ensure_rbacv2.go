package hooks

import "github.com/deckhouse/deckhouse/go_lib/hooks/ensure_rbacv2"

var _ = ensure_rbacv2.RegisterHook("flow-schema", []string{"kubernetes", "networking"}, nil)
