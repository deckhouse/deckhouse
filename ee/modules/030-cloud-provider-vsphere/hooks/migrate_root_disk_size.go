/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	migrateRootDiskSize "github.com/deckhouse/deckhouse/go_lib/hooks/migrate_root_disk_size"
)

var _ = migrateRootDiskSize.RegisterHook(
	&migrateRootDiskSize.HookParams{
		OldRootDiskSize:                       30,
		MasterRootDiskSizeFieldPath:           []string{"masterNodeGroup", "instanceClass", "rootDiskSize"},
		GenericNodeGroupRootDiskSizeFieldPath: []string{"instanceClass", "rootDiskSize"},
	},
)
