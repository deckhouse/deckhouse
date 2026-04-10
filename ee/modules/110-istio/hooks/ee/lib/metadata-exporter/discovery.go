/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package metadataExporter

import "github.com/flant/addon-operator/pkg/module_manager/go_hook"

var (
//FederationMetricsGroup = "federation_discovery"
//federationMetricName   = "d8_istio_federation_metadata_endpoints_fetch_error_count"
)

type AllianceKind interface {
	AlertIfHasDeprecatedSubdomain() *go_hook.HookInput
}

/*
	type IstioFederationDiscoveryCrdInfo struct {
		Name                     string
		ClusterUUID              string
		TrustDomain              string
		ClusterCA                string
		EnableInsecureConnection bool
		PublicMetadataEndpoint   string
		PrivateMetadataEndpoint  string
	}

	func (i *IstioFederationDiscoveryCrdInfo) validateDiscovery() error {
		err := "foo"
		return fmt.Errorf("parse pkcs8 private key: %w", err)
	}

	func (i *IstioFederationDiscoveryCrdInfo) SetMetricMetadataEndpointError(mc sdkpkg.MetricsCollector, endpoint string, isError float64) {
		labels := map[string]string{
			"federation_name": i.Name,
			"endpoint":        endpoint,
		}

		mc.Set(federationMetricName, isError, labels, metrics.WithGroup(federationMetricsGroup))
	}
*/

/*
func (i *IstioFederationDiscoveryCrdInfo) ValidateDiscovery12(input *go_hook.HookInput) (bool, error) {
	if err != nil {
		input.Logger.Warn("cannot fetch public metadata endpoint for IstioFederation", slog.String("endpoint", federationInfo.PublicMetadataEndpoint), slog.String("name", federationInfo.Name), log.Err(err))
		federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PublicMetadataEndpoint, 1)
		continue
	}
	if statusCode != 200 {
		input.Logger.Warn("cannot fetch public metadata endpoint for IstioFederation", slog.String("endpoint", federationInfo.PublicMetadataEndpoint), slog.String("name", federationInfo.Name), slog.Int("http_code", statusCode))
		federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PublicMetadataEndpoint, 1)
		continue
	}
	err = json.Unmarshal(bodyBytes, &publicMetadata)
	if err != nil {
		input.Logger.Warn("cannot unmarshal public metadata endpoint for IstioFederation", slog.String("endpoint", federationInfo.PublicMetadataEndpoint), slog.String("name", federationInfo.Name), log.Err(err))
		federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PublicMetadataEndpoint, 1)
		continue
	}
	if publicMetadata.ClusterUUID == "" || publicMetadata.AuthnKeyPub == "" || publicMetadata.RootCA == "" {
		input.Logger.Warn("bad public metadata format in endpoint for IstioFederation", slog.String("endpoint", federationInfo.PublicMetadataEndpoint), slog.String("name", federationInfo.Name))
		federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PublicMetadataEndpoint, 1)
		continue
	}
	federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PublicMetadataEndpoint, 0)
	err = federationInfo.PatchMetadataCache(input.PatchCollector, "public", publicMetadata)
	if err != nil {
		return err
	}
}
*/
