/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	ycmeta "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/meta"
	"github.com/deckhouse/deckhouse/go_lib/hooks/cloud_provider_migration_pending_metric"
	"github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal"
)

var _ = cloud_provider_migration_pending_metric.RegisterHook(
	internal.YandexMigrationConfigMapName,
	ycmeta.Namespace,
	"d8_cloud_provider_yandexcloud_migration_pending",
	"D8CloudProviderYandexCloudMigration",
)
