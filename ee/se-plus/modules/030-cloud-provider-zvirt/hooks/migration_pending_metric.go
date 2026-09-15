/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	"github.com/deckhouse/deckhouse/go_lib/hooks/cloud_provider_migration_pending_metric"
)

var _ = cloud_provider_migration_pending_metric.RegisterHook(
	cpapi.MigrationConfigMapName,
	internal.Namespace,
	"d8_cloud_provider_zvirt_migration_pending",
	"D8CloudProviderZvirtMigration",
)
