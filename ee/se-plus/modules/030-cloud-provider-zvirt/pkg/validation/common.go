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
	"strings"

	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"

	zpccv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/pcc/v1"
)

const (
	CodeProviderServerRequired  = "provider_server_required"
	CodeProviderServerInvalid   = "provider_server_invalid"
	CodeProviderCABundleInvalid = "provider_ca_bundle_invalid"
	CodeProviderCABundleIgnored = "provider_ca_bundle_ignored"
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
