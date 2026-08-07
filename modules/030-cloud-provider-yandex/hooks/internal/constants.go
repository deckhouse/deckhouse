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

const (
	ModuleName                   = "cloud-provider-yandex"
	Namespace                    = "d8-cloud-provider-yandex"
	ExporterCredentialSecretName = "d8-credentials-exporter"

	PCCSecretName                    = "d8-provider-cluster-configuration"
	PCCDiscoveryDataFilename         = "cloud-provider-discovery-data.json"
	PCCClusterConfigFilename         = "cloud-provider-cluster-configuration.yaml"
	CandiDiscoverySecretName         = "d8-candi-cloud-provider-discovery-data"
	YandexMigrationResourcesName     = "d8-migration-resources"
	YandexMigrationResourcesFilename = "resources.yaml"
	YandexMigrationConfigMapName     = "d8-module-is-migrating"

	// Defaults applied when a YandexClusterConfiguration leaves a field unset. They
	// are the defaults the pre-migration terraform code used, not the
	// YandexInstanceClass CRD defaults: leaving the fields empty would let the
	// apiserver default platformID to standard-v3 and diskType to network-hdd, which
	// replaces the boot and etcd disks of every cluster that never set them.
	defaultPlatformID     = "standard-v2"
	defaultDiskType       = "network-ssd"
	defaultDiskSizeGB     = 50
	defaultEtcdDiskSizeGB = 10
)
