/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal/crd"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/jwt"
)

var (
	federationMetricsGroup = "federation_discovery"
	federationMetricName   = "d8_istio_federation_metadata_endpoints_fetch_error_count"
)

type IstioFederationDiscoveryCrdInfo struct {
	Name                    string
	ClusterUUID             string
	TrustDomain             string
	PublicMetadataEndpoint  string
	PrivateMetadataEndpoint string
	//                       map[hostname]IP
	PublicServicesVirtualIPs map[string]string
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

type ipIterator struct {
	IPInUseMap map[string]bool
	Octet3     int
	Octet4     int
}

func (i *ipIterator) Next() (string, error) {
	i.Octet4++
	if i.Octet4 == 256 {
		i.Octet3++
		i.Octet4 = 0
	}
	if i.Octet3 == 256 {
		return "", fmt.Errorf("IP pool for ServiceEntries is over. Too many remote public services")
	}
	ip := fmt.Sprintf("169.254.%d.%d", i.Octet3, i.Octet4)

	if _, ok := i.IPInUseMap[ip]; ok {
		return i.Next()
	}
	return ip, nil
}

func applyFederationFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var federation crd.IstioFederation

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

	var psvips = make(map[string]string)
	if federation.Status.MetadataCache.Private != nil && federation.Status.MetadataCache.Private.PublicServices != nil {
		for _, ps := range *federation.Status.MetadataCache.Private.PublicServices {
			if ps.VirtualIP != "" {
				psvips[ps.Hostname] = ps.VirtualIP
			}
		}
	}
	return IstioFederationDiscoveryCrdInfo{
		Name:                     federation.GetName(),
		TrustDomain:              federation.Spec.TrustDomain,
		ClusterUUID:              clusterUUID,
		PublicMetadataEndpoint:   me + "/public/public.json",
		PrivateMetadataEndpoint:  me + "/private/federation.json",
		PublicServicesVirtualIPs: psvips,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("federation"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "federations",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "IstioFederation",
			FilterFunc: applyFederationFilter,
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
		input.LogEntry.Warnf("authn keypair for signing requests to remote metadata endpoints isn't generated yet, retry in 1min")
		return nil
	}

	var myTrustDomain = input.Values.Get("global.discovery.clusterDomain").String()

	var virtualIPInUseMap = make(map[string]bool)
	for _, federation := range input.Snapshots["federations"] {
		federationInfo := federation.(IstioFederationDiscoveryCrdInfo)
		if federationInfo.TrustDomain == myTrustDomain {
			continue
		}
		for _, vip := range federationInfo.PublicServicesVirtualIPs {
			virtualIPInUseMap[vip] = true
		}
	}
	var ipi = ipIterator{
		IPInUseMap: virtualIPInUseMap,
		Octet3:     0,
		Octet4:     0,
	}

	for _, federation := range input.Snapshots["federations"] {
		federationInfo := federation.(IstioFederationDiscoveryCrdInfo)
		if federationInfo.TrustDomain == myTrustDomain {
			continue
		}

		var publicMetadata crd.AlliancePublicMetadata
		var privateMetadata crd.FederationPrivateMetadata

		bodyBytes, statusCode, err := internal.HTTPGet(dc.GetHTTPClient(), federationInfo.PublicMetadataEndpoint, "")
		if err != nil {
			input.LogEntry.Warnf("cannot fetch public metadata endpoint %s for IstioFederation %s, error: %s", federationInfo.PublicMetadataEndpoint, federationInfo.Name, err.Error())
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PublicMetadataEndpoint, 1)
			continue
		}
		if statusCode != 200 {
			input.LogEntry.Warnf("cannot fetch public metadata endpoint %s for IstioFederation %s (HTTP Code %d)", federationInfo.PublicMetadataEndpoint, federationInfo.Name, statusCode)
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PublicMetadataEndpoint, 1)
			continue
		}
		err = json.Unmarshal(bodyBytes, &publicMetadata)
		if err != nil {
			input.LogEntry.Warnf("cannot unmarshal public metadata endpoint %s for IstioFederation %s, error: %s", federationInfo.PublicMetadataEndpoint, federationInfo.Name, err.Error())
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PublicMetadataEndpoint, 1)
			continue
		}
		if publicMetadata.ClusterUUID == "" || publicMetadata.AuthnKeyPub == "" || publicMetadata.RootCA == "" {
			input.LogEntry.Warnf("bad public metadata format in endpoint %s for IstioFederation %s", federationInfo.PublicMetadataEndpoint, federationInfo.Name)
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
			input.LogEntry.Warnf("can't generate auth token for endpoint %s of IstioFederation %s, error: %s", federationInfo.PrivateMetadataEndpoint, federationInfo.Name, err.Error())
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		bodyBytes, statusCode, err = internal.HTTPGet(dc.GetHTTPClient(), federationInfo.PrivateMetadataEndpoint, bearerToken)
		if err != nil {
			input.LogEntry.Warnf("cannot fetch private metadata endpoint %s for IstioFederation %s, error: %s", federationInfo.PrivateMetadataEndpoint, federationInfo.Name, err.Error())
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		if statusCode != 200 {
			input.LogEntry.Warnf("cannot fetch private metadata endpoint %s for IstioFederation %s (HTTP Code %d)", federationInfo.PrivateMetadataEndpoint, federationInfo.Name, statusCode)
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		err = json.Unmarshal(bodyBytes, &privateMetadata)
		if err != nil {
			input.LogEntry.Warnf("cannot unmarshal private metadata endpoint %s for IstioFederation %s, error: %s", federationInfo.PrivateMetadataEndpoint, federationInfo.Name, err.Error())
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		if privateMetadata.IngressGateways == nil || privateMetadata.PublicServices == nil {
			input.LogEntry.Warnf("bad private metadata format in endpoint %s for IstioFederation %s", federationInfo.PrivateMetadataEndpoint, federationInfo.Name)
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		for i, ps := range *privateMetadata.PublicServices {
			if vip, ok := federationInfo.PublicServicesVirtualIPs[ps.Hostname]; ok {
				(*privateMetadata.PublicServices)[i].VirtualIP = vip
			} else {
				(*privateMetadata.PublicServices)[i].VirtualIP, err = ipi.Next()
				if err != nil {
					input.LogEntry.Warnf("can't get VirtualIP for service %s in IstioFederation %s, error: %s", ps.Hostname, federationInfo.Name, err.Error())
					federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
					continue
				}
			}
		}
		federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 0)
		err = federationInfo.PatchMetadataCache(input.PatchCollector, "private", privateMetadata)
		if err != nil {
			return err
		}
	}
	return nil
}
