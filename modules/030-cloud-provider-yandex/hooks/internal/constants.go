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

package internal

import (
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

// The migration artifact names are the shared contract in go_lib/cloud-provider/api/migration.go:
// the admission webhook and the terraform projection look for exactly these objects, so they must
// never drift. Alias the shared constants instead of repeating the literals.
const (
	YandexMigrationResourcesName = cpapi.MigrationSecretName
	YandexMigrationConfigMapName = cpapi.MigrationConfigMapName
)

const (
	ModuleName                   = "cloud-provider-yandex"
	Namespace                    = "d8-cloud-provider-yandex"
	ExporterCredentialSecretName = "d8-credentials-exporter"

	PCCSecretName                    = "d8-provider-cluster-configuration"
	PCCDiscoveryDataFilename         = "cloud-provider-discovery-data.json"
	PCCClusterConfigFilename         = "cloud-provider-cluster-configuration.yaml"
	CandiDiscoverySecretName         = "d8-candi-cloud-provider-discovery-data"
	YandexMigrationResourcesFilename = "resources.yaml"

	// Defaults applied when a YandexClusterConfiguration leaves a field unset. They are the
	// defaults the pre-migration terraform code used, and deliberately not the values the
	// YandexInstanceClass v1 CRD documents (standard-v3 / network-hdd).
	//
	// The CRD carries no real `default:` for these fields - only x-doc-default - so the apiserver
	// does not fill them in; the fallbacks in candi/terraform-modules/{master,static}-node do,
	// and they use the documented v1 values. Projecting a PCC without these explicit defaults
	// would therefore hand terraform an absent field, it would fall back to standard-v3 /
	// network-hdd, and the boot and etcd disks of every cluster that never set them would be
	// replaced.
	defaultPlatformID     = "standard-v2"
	defaultDiskType       = "network-ssd"
	defaultDiskSizeGB     = 50
	defaultEtcdDiskSizeGB = 10
)
