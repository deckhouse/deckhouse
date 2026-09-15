/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

const (
	ModuleName = "cloud-provider-zvirt"
	Namespace  = "d8-cloud-provider-zvirt"

	PCCSecretName            = "d8-provider-cluster-configuration"
	PCCSecretNamespace       = "kube-system"
	PCCDiscoveryDataFilename = "cloud-provider-discovery-data.json"
	PCCClusterConfigFilename = "cloud-provider-cluster-configuration.yaml"

	MigrationResourcesFilename = "resources.yaml"

	// DefaultLayout is the only layout zVirt supports; a PCC always carries it, but the
	// projection needs a value when a hybrid cluster has no PCC at all.
	DefaultLayout = "Standard"

	// PlaceholderSSHPublicKey, PlaceholderServer and PlaceholderClusterID mark settings an admin has to replace by
	// hand.
	PlaceholderSSHPublicKey = "ssh-rsa PLACEHOLDER_REPLACE_ME"
	PlaceholderServer       = "PLACEHOLDER_REPLACE_ME"
	PlaceholderClusterID    = "00000000-0000-0000-0000-000000000000"
)
