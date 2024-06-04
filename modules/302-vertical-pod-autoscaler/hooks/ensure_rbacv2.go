package hooks

import "github.com/deckhouse/deckhouse/go_lib/hooks/ensure_rbacv2"

var _ = ensure_rbacv2.RegisterHook("vertical-pod-autoscaler", []string{"infrastructure"}, nil)
