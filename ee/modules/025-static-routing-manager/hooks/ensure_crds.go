/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/ensure_crds"
)

var (
	_ = ensure_crds.RegisterEnsureCRDsHook("/deckhouse/ee/modules/025-static-routing-manager/crds/*.yaml")
	_ = ensure_crds.RegisterEnsureCRDsHook("/deckhouse/ee/modules/025-static-routing-manager/crds/internal/*.yaml")
)
