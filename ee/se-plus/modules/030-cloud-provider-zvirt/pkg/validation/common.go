/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package validation

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/url"
	"slices"
	"strings"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"

	zpccv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/pcc/v1"
	zsettingsv2 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/settings/v2"
)

const (
	CodeProviderServerRequired  = "provider_server_required"
	CodeProviderServerInvalid   = "provider_server_invalid"
	CodeProviderCABundleInvalid = "provider_ca_bundle_invalid"
	CodeProviderCABundleIgnored = "provider_ca_bundle_ignored"

	CodeNetworkInterfaceAddressDuplicate      = "network_interface_address_duplicate"
	CodeNetworkInterfaceAddressesInsufficient = "network_interface_addresses_insufficient"
	CodeDNSServerDuplicate                    = "dns_server_duplicate"
	CodeCustomNetworkConfigUnknownNodeGroup   = "custom_network_config_unknown_node_group"
)

// ValidateProviderConnection checks the zVirt API connection settings of the ModuleConfig
func ValidateProviderConnection(state *State) cpvalapi.Result {
	result := cpvalapi.Result{}
	if state == nil || state.ModuleConfig == nil || state.ModuleConfig.Spec.Settings == nil {
		return result
	}

	providerParams := state.ModuleConfig.Spec.Settings.Provider.Parameters

	pathPrefix := "ModuleConfig.spec.settings.provider.parameters"
	result.Merge(
		validateProviderServer(pathPrefix, providerParams.Server),
		validateProviderCABundleAndInsecureFlag(pathPrefix, providerParams.CABundle, providerParams.Insecure),
	)

	return result
}

// ValidateLegacyProviderConnection checks the same connection settings in the legacy
// ZvirtClusterConfiguration, which is still the source of truth until the migration completes.
func ValidateLegacyProviderConnection(pcc *zpccv1.ZvirtProviderClusterConfiguration) cpvalapi.Result {
	result := cpvalapi.Result{}
	if pcc == nil {
		return result
	}

	pathPrefix := "ProviderClusterConfiguration.provider"
	result.Merge(
		validateProviderServer(pathPrefix, pcc.Provider.Server),
		validateProviderCABundleAndInsecureFlag(pathPrefix, pcc.Provider.CABundle, pcc.Provider.Insecure),
	)

	return result
}

// ValidateLegacyCustomNetworkConfigParameters checks each static network configuration of the legacy
// ZvirtClusterConfiguration on its own, without looking at the node group it is attached to.
//
// The legacy addresses are one list and the DNS servers are one space-separated string; the
// migration copies both into the settings map verbatim, so a repeated entry is just as broken here
// as it is in the map.
func ValidateLegacyCustomNetworkConfigParameters(pcc *zpccv1.ZvirtProviderClusterConfiguration) cpvalapi.Result {
	result := cpvalapi.Result{}

	for _, entry := range getLegacyCustomNetworkConfigs(pcc) {
		result.Merge(
			validateUniqueValues(
				entry.pathPrefix+".networkInterfaceAddress",
				CodeNetworkInterfaceAddressDuplicate,
				"networkInterfaceAddress",
				entry.config.NetworkInterfaceAddress,
			),
			validateUniqueValues(
				entry.pathPrefix+".dnsServers",
				CodeDNSServerDuplicate,
				"dnsServers",
				strings.Fields(entry.config.DNSServers),
			),
		)
	}

	return result
}

// ValidateLegacyCustomNetworkConfigsCoverNodeGroupReplicas checks every static network configuration
// of the legacy ZvirtClusterConfiguration against the replicas of the node group that declares it.
//
// The migration projects the legacy replica count onto both bounds of the NodeGroup and hands the
// addresses out by node index, so a list shorter than the replica count leaves the surplus nodes on
// DHCP — the same silent failure ValidateCustomNetworkConfigsCoverNodeGroupReplicas catches after
// the migration, reported here before the doomed apply starts.
func ValidateLegacyCustomNetworkConfigsCoverNodeGroupReplicas(pcc *zpccv1.ZvirtProviderClusterConfiguration) cpvalapi.Result {
	result := cpvalapi.Result{}

	for _, entry := range getLegacyCustomNetworkConfigs(pcc) {
		addresses := entry.config.NetworkInterfaceAddress
		if entry.replicas <= len(addresses) {
			continue
		}

		result.AddError(
			entry.pathPrefix+".networkInterfaceAddress",
			CodeNetworkInterfaceAddressesInsufficient,
			len(addresses),
			fmt.Sprintf(
				"number of nodes in NodeGroup %q (%d) should be less than or equal to the length of %s.networkInterfaceAddress (%d)",
				entry.nodeGroupName,
				entry.replicas,
				entry.pathPrefix,
				len(addresses),
			),
		)
	}

	return result
}

// ValidateCustomNetworkConfigParameters checks each static network configuration on its own, without
// looking at the NodeGroups it is keyed by.
func ValidateCustomNetworkConfigParameters(state *State) cpvalapi.Result {
	result := cpvalapi.Result{}

	for nodeGroupName, config := range getCustomNetworkConfigs(state) {
		pathPrefix := fmt.Sprintf("ModuleConfig.spec.settings.nodes.parameters.customNetworkConfigs[%s]", nodeGroupName)

		result.Merge(
			validateUniqueValues(
				pathPrefix+".networkInterfaceAddresses",
				CodeNetworkInterfaceAddressDuplicate,
				"networkInterfaceAddresses",
				config.NetworkInterfaceAddresses,
			),
			validateUniqueValues(
				pathPrefix+".dnsServers",
				CodeDNSServerDuplicate,
				"dnsServers",
				config.DNSServers,
			),
		)
	}

	return result
}

// ValidateCustomNetworkConfigsCoverNodeGroupReplicas checks every customNetworkConfigs entry against
// the CloudPermanent NodeGroup it is keyed by.
//
// The address is picked by node index — try(...[nodeIndex], "") in the master-node and static-node
// terraform modules — so a list shorter than the replica count does not fail the apply: the
// surplus nodes silently come up without the static configuration the operator asked for.
//
// allNodeGroupsKnown says whether the state carries every CloudPermanent NodeGroup of the cluster:
// the ModuleConfig surface and preflight load all of them, while the NodeGroup surface sees only
// the group being reviewed. It gates the warning alone — a key the state cannot match is not
// reported as naming nothing when its siblings were never loaded.
func ValidateCustomNetworkConfigsCoverNodeGroupReplicas(state *State, allNodeGroupsKnown bool) cpvalapi.Result {
	result := cpvalapi.Result{}

	for nodeGroupName, networkConfig := range getCustomNetworkConfigs(state) {
		pathPrefix := fmt.Sprintf("ModuleConfig.spec.settings.nodes.parameters.customNetworkConfigs[%s]", nodeGroupName)

		nodeGroup, ok := state.FindNodeGroup(nodeGroupName)
		if !ok {
			if allNodeGroupsKnown {
				result.AddWarning(
					pathPrefix,
					CodeCustomNetworkConfigUnknownNodeGroup,
					nodeGroupName,
					fmt.Sprintf(
						"customNetworkConfigs contains the name of a non-existent NodeGroup %q",
						nodeGroupName,
					),
				)
			}

			continue
		}

		if nodeGroup.Spec.NodeType != cpapi.NodeTypeCloudPermanent {
			if allNodeGroupsKnown {
				result.AddWarning(
					pathPrefix,
					CodeCustomNetworkConfigUnknownNodeGroup,
					nodeGroupName,
					fmt.Sprintf(
						"customNetworkConfigs contains the name of NodeGroup %q, which is not CloudPermanent",
						nodeGroupName,
					),
				)
			}

			continue
		}

		if nodeGroup.Spec.CloudInstances == nil {
			continue
		}

		replicas := nodeGroup.Spec.CloudInstances.MaxPerZone
		if replicas <= len(networkConfig.NetworkInterfaceAddresses) {
			continue
		}

		result.AddError(
			pathPrefix+".networkInterfaceAddresses",
			CodeNetworkInterfaceAddressesInsufficient,
			len(networkConfig.NetworkInterfaceAddresses),
			fmt.Sprintf(
				"number of nodes in NodeGroup %q (%d) should be less than or equal to the length of settings.nodes.parameters.customNetworkConfigs[%s].networkInterfaceAddresses (%d)",
				nodeGroup.Name,
				replicas,
				nodeGroup.Name,
				len(networkConfig.NetworkInterfaceAddresses),
			),
		)
	}

	return result
}

// getCustomNetworkConfigs returns the configured map, or nothing when the ModuleConfig carries no
// settings yet — an absent ModuleConfig is reported by its own rule, not by these.
func getCustomNetworkConfigs(state *State) map[string]zsettingsv2.CustomNetworkConfig {
	if state == nil || state.ModuleConfig == nil || state.ModuleConfig.Spec.Settings == nil {
		return nil
	}

	return state.ModuleConfig.Spec.Settings.Nodes.Parameters.CustomNetworkConfigs
}

// legacyCustomNetworkConfig is one static network configuration of the legacy configuration,
// together with the node group it belongs to and the path to report it under.
type legacyCustomNetworkConfig struct {
	nodeGroupName string
	pathPrefix    string
	replicas      int
	config        *zpccv1.ZvirtNetworkConfig
}

// getLegacyCustomNetworkConfigs lists the static network configurations the legacy configuration
// declares: one on the masterNodeGroup, and one per additional node group that sets it. The legacy
// configuration attaches the configuration to the InstanceClass, which is why the path crosses it.
func getLegacyCustomNetworkConfigs(pcc *zpccv1.ZvirtProviderClusterConfiguration) []legacyCustomNetworkConfig {
	if pcc == nil {
		return nil
	}

	entries := make([]legacyCustomNetworkConfig, 0, len(pcc.NodeGroups)+1)

	if config := pcc.MasterNodeGroup.InstanceClass.CustomNetworkConfig; config != nil {
		entries = append(entries, legacyCustomNetworkConfig{
			nodeGroupName: "master",
			pathPrefix:    "ProviderClusterConfiguration.masterNodeGroup.instanceClass.customNetworkConfig",
			replicas:      pcc.MasterNodeGroup.Replicas,
			config:        config,
		})
	}

	for _, nodeGroup := range pcc.NodeGroups {
		if nodeGroup.InstanceClass.CustomNetworkConfig == nil {
			continue
		}

		entries = append(entries, legacyCustomNetworkConfig{
			nodeGroupName: nodeGroup.Name,
			pathPrefix: fmt.Sprintf(
				"ProviderClusterConfiguration.nodeGroups[%s].instanceClass.customNetworkConfig",
				nodeGroup.Name,
			),
			replicas: nodeGroup.Replicas,
			config:   nodeGroup.InstanceClass.CustomNetworkConfig,
		})
	}

	return entries
}

// validateUniqueValues reports the repeated values of a list as a single violation naming all of
// them. Result keys violations by code and path, so one error per duplicate would collapse into
// the last one anyway — and an operator fixing a long address list wants to see every duplicate at
// once rather than one per apply.
func validateUniqueValues(path, code, fieldName string, values []string) cpvalapi.Result {
	result := cpvalapi.Result{}

	seen := make(map[string]struct{}, len(values))
	duplicates := make([]string, 0)
	for _, value := range values {
		if _, ok := seen[value]; !ok {
			seen[value] = struct{}{}
			continue
		}

		if !slices.Contains(duplicates, value) {
			duplicates = append(duplicates, value)
		}
	}

	if len(duplicates) == 0 {
		return result
	}

	result.AddError(
		path,
		code,
		duplicates,
		fmt.Sprintf("%s must not contain duplicates, listed more than once: %s", fieldName, strings.Join(duplicates, ", ")),
	)

	return result
}

// validateProviderServer checks the endpoint both the legacy configuration and the ModuleConfig
// declare, so the two are held to one standard. A configuration that passes preflight and then
// fails admission after the migration is the failure mode this guards against.
func validateProviderServer(pathPrefix, server string) cpvalapi.Result {
	result := cpvalapi.Result{}

	serverPath := pathPrefix + ".server"
	switch {
	case strings.TrimSpace(server) == "":
		result.AddError(
			serverPath,
			CodeProviderServerRequired,
			nil,
			"zVirt API endpoint is required",
		)
	default:
		if err := validateServerURL(server); err != nil {
			result.AddError(
				serverPath,
				CodeProviderServerInvalid,
				server,
				fmt.Sprintf("invalid zVirt API endpoint: %v", err),
			)
		}
	}

	return result
}

// validateServerURL accepts the endpoint the ovirt provider and the discoverer are given verbatim.
func validateServerURL(server string) error {
	parsed, err := url.Parse(server)
	if err != nil {
		return err
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https, got %q", parsed.Scheme)
	}

	if parsed.Host == "" {
		return fmt.Errorf("host is missing")
	}

	return nil
}

// validateProviderCABundleAndInsecureFlag checks the CA bundle and how it sits with the insecure
// flag. An absent bundle is fine — the system trust store is then used — so only a supplied one is
// checked, and combining it with insecure is reported as a warning rather than an error.
func validateProviderCABundleAndInsecureFlag(pathPrefix, caBundle string, insecure bool) cpvalapi.Result {
	result := cpvalapi.Result{}

	if caBundle == "" {
		return result
	}

	caBundlePath := pathPrefix + ".caBundle"
	if err := validateCABundle(caBundle); err != nil {
		result.AddError(
			caBundlePath,
			CodeProviderCABundleInvalid,
			"masked",
			fmt.Sprintf("invalid CA bundle: %v", err),
		)
		return result
	}

	// Not an error: the cluster works, it just does not verify the certificate it was given.
	// Silently ignoring a CA bundle the operator went to the trouble of supplying is worse.
	if insecure {
		result.AddWarning(
			caBundlePath,
			CodeProviderCABundleIgnored,
			"masked",
			"caBundle is ignored because insecure is set to true",
		)
	}

	return result
}

// validateCABundle accepts a base64-encoded PEM bundle: that is what terraform hands to the ovirt
// provider through base64decode, and what the templates mount for the in-cluster components.
func validateCABundle(caBundle string) error {
	decodedCABundle, err := base64.StdEncoding.DecodeString(caBundle)
	if err != nil {
		return fmt.Errorf("value is not valid base64: %w", err)
	}

	rest := decodedCABundle
	certCount := 0
	for {
		// Decode the PEM string
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}

		// Ensure it's a certificate block
		if block.Type != "CERTIFICATE" {
			return fmt.Errorf("unexpected PEM block %q, expected CERTIFICATE", block.Type)
		}

		// Parse the DER-encoded certificate bytes
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse certificate: %w", err)
		}

		// Return whether the certificate is a Certificate Authority
		if !cert.IsCA {
			return fmt.Errorf("certificate is not a CA")
		}

		certCount++
	}

	if certCount == 0 {
		return fmt.Errorf("no PEM CERTIFICATE block found")
	}

	return nil
}
