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

	CandiDiscoverySecretName = "d8-candi-cloud-provider-discovery-data"

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

// DiscoveryDataSchemaPaths are the directories the ZvirtCloudProviderDiscoveryData schema is
// looked up in. zVirt is an external cloud provider, so tools/build.go no longer bakes its candi
// into /deckhouse/candi/cloud-providers/zvirt — that tree ships in the OCI bundle dhctl unpacks at
// runtime, which this controller never sees. The schema reaches the image inside the module
// itself; under test /deckhouse is the repository, where the module is still at its ee/se-plus
// path. A directory that does not exist is skipped silently, and an empty set of schemas is what
// raises SchemaNotFound.
var DiscoveryDataSchemaPaths = []string{
	"/deckhouse/candi/cloud-providers/zvirt/openapi",
	"/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/candi/openapi",
	"/deckhouse/modules/030-cloud-provider-zvirt/candi/openapi",
}
