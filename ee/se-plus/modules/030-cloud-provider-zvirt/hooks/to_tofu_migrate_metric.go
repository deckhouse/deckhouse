/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	metric "github.com/deckhouse/deckhouse/go_lib/hooks/to_tofu_migrate_metric"
)

var _ = metric.RegisterHook()
