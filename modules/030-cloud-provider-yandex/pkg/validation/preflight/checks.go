// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	"fmt"
	"regexp"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"

	ycmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/meta"
	ycval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/validation"
)

// Validation violation codes for preflight-only checks.
const (
	CodeInvalidClusterPrefix = "cluster_prefix_invalid"
)

var (
	// prefixRegex guards the cluster prefix, which becomes the name prefix of every cloud resource
	// the layouts create. Yandex Cloud rejects names that do not match it, so an invalid prefix has
	// to fail here, before any infrastructure is touched, rather than mid-apply on the first resource.
	prefixRegex = regexp.MustCompile("^([a-z]([-a-z0-9]{0,61}[a-z0-9])?)$")
)

// ValidatePreflight checks resources required before cluster bootstrap or converge.
func ValidatePreflight(state *ycval.State, operation validatev1.Operation, clusterPrefix string) cpvalapi.Result {
	if state == nil {
		return cpvalapi.ResultForNilState()
	}

	result := cpvalapi.Result{}

	// Common checks: they apply to every cluster, migrated or not, so they run before the
	// migration gate below rather than inside one of the two model-specific branches.
	result.Merge(validateClusterPrefix(clusterPrefix))

	// Validate legacy ProviderClusterConfiguration.
	if state.HasProviderClusterConfig() {
		result.Merge(
			ycval.ValidatePCCServiceAccount(state.ProviderClusterConfig),
			ycval.ValidatePCCNodeGroupsReplicasAndIPAddresses(state.ProviderClusterConfig),
			ycval.ValidatePCCWithNATInstanceLayout(state.ProviderClusterConfig, operation),
			ycval.ValidatePCCDiskSizeByType(state.ProviderClusterConfig),
		)
	}

	if cpapi.ShouldSkipNewModelValidation(state.MigrationStatus) {
		return result
	}

	// Validate new resources: ModuleConfig, NodeGroup, InstanceClasses, CredentialSecrets.
	result.Merge(
		cpval.ValidateModuleConfig(state),
		cpval.ValidateCredentialSecretPresence(state, cpapi.CredentialSecretName),
		cpval.ValidateCredentialSecretContent(state, cpapi.CredentialSecretName, ycval.CredentialsValidator),
		cpval.ValidateCredentialSecretContent(state, ycmeta.ExporterCredentialSecretName, ycval.ExporterCredentialsValidator),
		cpval.ValidateMasterNodeGroupPresence(state),
		cpval.ValidateNodeGroupsClassReference(state, true),
		cpval.ValidateInstanceClassesEtcdDisk(state),
		ycval.ValidateNodeGroupExternalIPAddresses(state),
		ycval.ValidateWithNATInstanceLayout(state),
		ycval.ValidateProvisionedStorageClasses(state),
		ycval.ValidateDiskSizeByType(state),
	)

	return result
}

func validateClusterPrefix(prefix string) cpvalapi.Result {
	result := cpvalapi.Result{}

	if !prefixRegex.MatchString(prefix) {
		result.AddError(
			"ClusterConfiguration.cloud.prefix",
			CodeInvalidClusterPrefix,
			prefix,
			fmt.Sprintf("invalid prefix %q, prefix must match the pattern: %s", prefix, prefixRegex.String()),
		)
	}

	return result
}
