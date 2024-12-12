/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	eeCrd "github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/ee/lib/crd"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/http"
	"github.com/deckhouse/deckhouse/go_lib/jwt"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
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

func (i *IstioFederationDiscoveryCrdInfo) SetMetricMetadataEndpointError(mc go_hook.MetricsCollector, endpoint string, isError float64) {
	labels := map[string]string{
		"federation_name": i.Name,
		"endpoint":        endpoint,
	}

	mc.Set(federationMetricName, isError, labels, metrics.WithGroup(federationMetricsGroup))
}

func (i *IstioFederationDiscoveryCrdInfo) PatchMetadataCache(pc *object_patch.PatchCollector, scope string, meta interface{}) error {
	patch := map[string]interface{}{
		"status": map[string]interface{}{
			"metadataCache": map[string]interface{}{
				scope:                        meta,
				scope + "LastFetchTimestamp": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	pc.MergePatch(patch, "deckhouse.io/v1alpha1", "IstioFederation", "", i.Name, object_patch.WithSubresource("/status"))
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

func federationDiscovery(input *go_hook.HookInput, dc dependency.Container) error {
	input.MetricsCollector.Expire(federationMetricsGroup)

	if !input.Values.Get("istio.federation.enabled").Bool() {
		return nil
	}
	if !input.Values.Get("istio.internal.remoteAuthnKeypair.priv").Exists() {
		input.Logger.Warnf("authn keypair for signing requests to remote metadata endpoints isn't generated yet, retry in 1min")
		return nil
	}

	var myTrustDomain = input.Values.Get("global.discovery.clusterDomain").String()

	for _, federation := range input.Snapshots["federations"] {
		federationInfo := federation.(IstioFederationDiscoveryCrdInfo)
		if federationInfo.TrustDomain == myTrustDomain {
			continue
		}
	}

	for _, federation := range input.Snapshots["federations"] {
		federationInfo := federation.(IstioFederationDiscoveryCrdInfo)
		if federationInfo.TrustDomain == myTrustDomain {
			continue
		}

		var publicMetadata eeCrd.AlliancePublicMetadata
		var privateMetadata eeCrd.FederationPrivateMetadata
		var httpOption []http.Option

		if federationInfo.ClusterCA != "" {
			caCerts := [][]byte{[]byte(federationInfo.ClusterCA)}
			httpOption = append(httpOption, http.WithAdditionalCACerts(caCerts))
		} else if federationInfo.EnableInsecureConnection {
			httpOption = append(httpOption, http.WithInsecureSkipVerify())
		}

		bodyBytes, statusCode, err := lib.HTTPGet(dc.GetHTTPClient(httpOption...), federationInfo.PublicMetadataEndpoint, "")
		if err != nil {
			input.Logger.Warnf("cannot fetch public metadata endpoint %s for IstioFederation %s, error: %s", federationInfo.PublicMetadataEndpoint, federationInfo.Name, err.Error())
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PublicMetadataEndpoint, 1)
			continue
		}
		if statusCode != 200 {
			input.Logger.Warnf("cannot fetch public metadata endpoint %s for IstioFederation %s (HTTP Code %d)", federationInfo.PublicMetadataEndpoint, federationInfo.Name, statusCode)
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PublicMetadataEndpoint, 1)
			continue
		}
		err = json.Unmarshal(bodyBytes, &publicMetadata)
		if err != nil {
			input.Logger.Warnf("cannot unmarshal public metadata endpoint %s for IstioFederation %s, error: %s", federationInfo.PublicMetadataEndpoint, federationInfo.Name, err.Error())
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PublicMetadataEndpoint, 1)
			continue
		}
		if publicMetadata.ClusterUUID == "" || publicMetadata.AuthnKeyPub == "" || publicMetadata.RootCA == "" {
			input.Logger.Warnf("bad public metadata format in endpoint %s for IstioFederation %s", federationInfo.PublicMetadataEndpoint, federationInfo.Name)
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
			input.Logger.Warnf("can't generate auth token for endpoint %s of IstioFederation %s, error: %s", federationInfo.PrivateMetadataEndpoint, federationInfo.Name, err.Error())
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		bodyBytes, statusCode, err = lib.HTTPGet(dc.GetHTTPClient(httpOption...), federationInfo.PrivateMetadataEndpoint, bearerToken)
		if err != nil {
			input.Logger.Warnf("cannot fetch private metadata endpoint %s for IstioFederation %s, error: %s", federationInfo.PrivateMetadataEndpoint, federationInfo.Name, err.Error())
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		if statusCode != 200 {
			input.Logger.Warnf("cannot fetch private metadata endpoint %s for IstioFederation %s (HTTP Code %d)", federationInfo.PrivateMetadataEndpoint, federationInfo.Name, statusCode)
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		err = json.Unmarshal(bodyBytes, &privateMetadata)
		if err != nil {
			input.Logger.Warnf("cannot unmarshal private metadata endpoint %s for IstioFederation %s, error: %s", federationInfo.PrivateMetadataEndpoint, federationInfo.Name, err.Error())
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		if privateMetadata.IngressGateways == nil || privateMetadata.PublicServices == nil {
			input.Logger.Warnf("bad private metadata format in endpoint %s for IstioFederation %s", federationInfo.PrivateMetadataEndpoint, federationInfo.Name)
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 0)
		err = federationInfo.PatchMetadataCache(input.PatchCollector, "private", privateMetadata)
		if err != nil {
			return err
		}
	}
	return nil
}
