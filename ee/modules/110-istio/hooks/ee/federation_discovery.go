/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	eeCrd "github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/ee/lib/crd"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/http"
	"github.com/deckhouse/deckhouse/go_lib/jwt"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
	"github.com/deckhouse/deckhouse/pkg/log"
)

var (
	federationMetricsGroup = "federation_discovery"
	federationMetricName   = "d8_istio_federation_metadata_endpoints_fetch_error_count"
)

type IstioFederationDiscoveryCrdInfo struct {
	Name                     string
	ClusterUUID              string
	TrustDomain              string
	ClusterCA                string
	EnableInsecureConnection bool
	PublicMetadataEndpoint   string
	PrivateMetadataEndpoint  string
}

func (i *IstioFederationDiscoveryCrdInfo) SetMetricMetadataEndpointError(mc sdkpkg.MetricsCollector, endpoint string, isError float64) {
	labels := map[string]string{
		"federation_name": i.Name,
		"endpoint":        endpoint,
	}

	mc.Set(federationMetricName, isError, labels, metrics.WithGroup(federationMetricsGroup))
}

func (i *IstioFederationDiscoveryCrdInfo) PatchMetadataCache(pc go_hook.PatchCollector, scope string, meta interface{}) error {
	patch := map[string]interface{}{
		"status": map[string]interface{}{
			"metadataCache": map[string]interface{}{
				scope:                        meta,
				scope + "LastFetchTimestamp": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	pc.PatchWithMerge(patch, "deckhouse.io/v1alpha1", "IstioFederation", "", i.Name, object_patch.WithSubresource("/status"))
	return nil
}

func applyFederationFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var federation eeCrd.IstioFederation

	err := sdk.FromUnstructured(obj, &federation)
	if err != nil {
		return nil, err
	}

	clusterUUID := ""
	if federation.Status.MetadataCache.Public != nil {
		clusterUUID = federation.Status.MetadataCache.Public.ClusterUUID
	}

	me := federation.Spec.MetadataEndpoint
	me = strings.TrimSuffix(me, "/")

	return IstioFederationDiscoveryCrdInfo{
		Name:                     federation.GetName(),
		TrustDomain:              federation.Spec.TrustDomain,
		ClusterCA:                federation.Spec.Metadata.ClusterCA,
		EnableInsecureConnection: federation.Spec.Metadata.EnableInsecureConnection,
		ClusterUUID:              clusterUUID,
		PublicMetadataEndpoint:   me + "/public/public.json",
		PrivateMetadataEndpoint:  me + "/private/federation.json",
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: lib.Queue("federation"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "federations",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "IstioFederation",
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   applyFederationFilter,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "* * * * *"},
	},
}, dependency.WithExternalDependencies(federationDiscovery))

func federationDiscovery(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	input.MetricsCollector.Expire(federationMetricsGroup)

	if !input.Values.Get("istio.federation.enabled").Bool() {
		return nil
	}
	if !input.Values.Get("istio.internal.remoteAuthnKeypair.priv").Exists() {
		input.Logger.Warn("authn keypair for signing requests to remote metadata endpoints isn't generated yet, retry in 1min")
		return nil
	}

	var myTrustDomain = input.Values.Get("global.discovery.clusterDomain").String()

	federations, err := sdkobjectpatch.UnmarshalToStruct[IstioFederationDiscoveryCrdInfo](input.Snapshots, "federations")
	if err != nil {
		return fmt.Errorf("failed to unmarshal federations snapshot: %w", err)
	}

	for _, federationInfo := range federations {
		if federationInfo.TrustDomain == myTrustDomain {
			continue
		}

		var publicMetadata eeCrd.AlliancePublicMetadata
		var privateMetadata eeCrd.FederationPrivateMetadata
		var httpOption []http.Option
		protocolMap := map[string]string{
			"https":    "TLS",
			"tls":      "TLS",
			"http":     "HTTP",
			"http2":    "HTTP2",
			"grpc":     "HTTP2",
			"grpc-web": "HTTP2",
		}

		defaultProtocol := "TCP"

		if federationInfo.ClusterCA != "" {
			caCerts := [][]byte{[]byte(federationInfo.ClusterCA)}
			httpOption = append(httpOption, http.WithAdditionalCACerts(caCerts))
		} else if federationInfo.EnableInsecureConnection {
			httpOption = append(httpOption, http.WithInsecureSkipVerify())
		}

		bodyBytes, statusCode, err := lib.HTTPGet(dc.GetHTTPClient(httpOption...), federationInfo.PublicMetadataEndpoint, "")
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

		// TODO Make independent public and private fetch?
		privKey := []byte(input.Values.Get("istio.internal.remoteAuthnKeypair.priv").String())
		claims := map[string]string{
			"iss":   "d8-istio",
			"aud":   publicMetadata.ClusterUUID,
			"sub":   input.Values.Get("global.discovery.clusterUUID").String(),
			"scope": "private-federation",
		}
		bearerToken, err := jwt.GenerateJWT(privKey, claims, time.Minute)
		if err != nil {
			input.Logger.Warn("can't generate auth token for endpoint of IstioFederation", slog.String("endpoint", federationInfo.PrivateMetadataEndpoint), slog.String("name", federationInfo.Name), log.Err(err))
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		bodyBytes, statusCode, err = lib.HTTPGet(dc.GetHTTPClient(httpOption...), federationInfo.PrivateMetadataEndpoint, bearerToken)
		if err != nil {
			input.Logger.Warn("cannot fetch private metadata endpoint for IstioFederation", slog.String("endpoint", federationInfo.PrivateMetadataEndpoint), slog.String("name", federationInfo.Name), log.Err(err))
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		if statusCode != 200 {
			input.Logger.Warn("cannot fetch private metadata endpoint for IstioFederation", slog.String("endpoint", federationInfo.PrivateMetadataEndpoint), slog.String("name", federationInfo.Name), slog.Int("http_code", statusCode))
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		err = json.Unmarshal(bodyBytes, &privateMetadata)
		if err != nil {
			input.Logger.Warn("cannot unmarshal private metadata endpoint for IstioFederation", slog.String("endpoint", federationInfo.PrivateMetadataEndpoint), slog.String("name", federationInfo.Name), log.Err(err))
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		if privateMetadata.IngressGateways == nil || privateMetadata.PublicServices == nil {
			input.Logger.Warn("bad private metadata format in endpoint for IstioFederation", slog.String("endpoint", federationInfo.PrivateMetadataEndpoint), slog.String("name", federationInfo.Name))
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 0)
		updatePortProtocols(privateMetadata.PublicServices, defaultProtocol, protocolMap)
		err = federationInfo.PatchMetadataCache(input.PatchCollector, "private", privateMetadata)
		if err != nil {
			return err
		}
	}
	return nil
}

func updatePortProtocols(services *[]eeCrd.FederationPublicServices, defaultProtocol string, protocolMap map[string]string) {
	keys := make([]string, 0, len(protocolMap))
	for key := range protocolMap {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b string) int { return len(b) - len(a) })
	for serviceIndex := range *services {
		service := &(*services)[serviceIndex]
		for portIndex, port := range service.Ports {
			port.Protocol = defaultProtocol
			portNameParts := strings.SplitN(port.Name, "-", 2)
			basePortName := portNameParts[0]
			for _, keyword := range keys {
				protocol := protocolMap[keyword]
				if strings.Contains(basePortName, keyword) {
					port.Protocol = protocol
					break
				}
			}
			service.Ports[portIndex] = port
		}
	}
}
