/*
Copyright 2022 Flant JSC
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

	eeCrd "github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/ee/lib/crd"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/jwt"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

var (
	multiclusterMetricsGroup = "multicluster_discovery"
	multiclusterMetricName   = "d8_istio_multicluster_metadata_endpoints_fetch_error_count"
)

type IstioMulticlusterDiscoveryCrdInfo struct {
	Name                    string
	ClusterUUID             string
	EnableIngressGateway    bool
	PublicMetadataEndpoint  string
	PrivateMetadataEndpoint string
}

func (i *IstioMulticlusterDiscoveryCrdInfo) SetMetricMetadataEndpointError(mc go_hook.MetricsCollector, endpoint string, isError float64) {
	labels := map[string]string{
		"multicluster_name": i.Name,
		"endpoint":          endpoint,
	}

	mc.Set(multiclusterMetricName, isError, labels, metrics.WithGroup(multiclusterMetricsGroup))
}

func (i *IstioMulticlusterDiscoveryCrdInfo) PatchMetadataCache(pc *object_patch.PatchCollector, scope string, meta interface{}) error {
	patch := map[string]interface{}{
		"status": map[string]interface{}{
			"metadataCache": map[string]interface{}{
				scope:                        meta,
				scope + "LastFetchTimestamp": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	pc.MergePatch(patch, "deckhouse.io/v1alpha1", "IstioMulticluster", "", i.Name, object_patch.WithSubresource("/status"))
	return nil
}

func applyMulticlusterFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var multicluster eeCrd.IstioMulticluster

	err := sdk.FromUnstructured(obj, &multicluster)
	if err != nil {
		return nil, err
	}

	clusterUUID := ""
	if multicluster.Status.MetadataCache.Public != nil {
		clusterUUID = multicluster.Status.MetadataCache.Public.ClusterUUID
	}

	me := multicluster.Spec.MetadataEndpoint
	me = strings.TrimSuffix(me, "/")

	return IstioMulticlusterDiscoveryCrdInfo{
		Name:                    multicluster.GetName(),
		EnableIngressGateway:    multicluster.Spec.EnableIngressGateway,
		ClusterUUID:             clusterUUID,
		PublicMetadataEndpoint:  me + "/public/public.json",
		PrivateMetadataEndpoint: me + "/private/multicluster.json",
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: lib.Queue("multicluster"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "multiclusters",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "IstioMulticluster",
			FilterFunc: applyMulticlusterFilter,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "* * * * *"},
	},
}, dependency.WithExternalDependencies(multiclusterDiscovery))

func multiclusterDiscovery(input *go_hook.HookInput, dc dependency.Container) error {
	input.MetricsCollector.Expire(multiclusterMetricsGroup)

	if !input.Values.Get("istio.multicluster.enabled").Bool() {
		return nil
	}
	if !input.Values.Get("istio.internal.remoteAuthnKeypair.priv").Exists() {
		input.LogEntry.Warnf("authn keypair for signing requests to remote metadata endpoints isn't generated yet, retry in 1min")
		return nil
	}

	for _, multicluster := range input.Snapshots["multiclusters"] {
		multiclusterInfo := multicluster.(IstioMulticlusterDiscoveryCrdInfo)

		var publicMetadata eeCrd.AlliancePublicMetadata
		var privateMetadata eeCrd.MulticlusterPrivateMetadata

		bodyBytes, statusCode, err := lib.HTTPGet(dc.GetHTTPClient(), multiclusterInfo.PublicMetadataEndpoint, "")
		if err != nil {
			input.LogEntry.Warnf("cannot fetch public metadata endpoint %s for IstioMulticluster %s, error: %s", multiclusterInfo.PublicMetadataEndpoint, multiclusterInfo.Name, err.Error())
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			continue
		}
		if statusCode != 200 {
			input.LogEntry.Warnf("cannot fetch public metadata endpoint %s for IstioMulticluster %s (HTTP Code %d)", multiclusterInfo.PublicMetadataEndpoint, multiclusterInfo.Name, statusCode)
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			continue
		}
		err = json.Unmarshal(bodyBytes, &publicMetadata)
		if err != nil {
			input.LogEntry.Warnf("cannot unmarshal public metadata endpoint %s for IstioMulticluster %s, error: %s", multiclusterInfo.PublicMetadataEndpoint, multiclusterInfo.Name, err.Error())
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			continue
		}
		if publicMetadata.ClusterUUID == "" || publicMetadata.AuthnKeyPub == "" || publicMetadata.RootCA == "" {
			input.LogEntry.Warnf("bad public metadata format in endpoint %s for IstioMulticluster %s", multiclusterInfo.PublicMetadataEndpoint, multiclusterInfo.Name)
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			continue
		}
		multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 0)
		err = multiclusterInfo.PatchMetadataCache(input.PatchCollector, "public", publicMetadata)
		if err != nil {
			return err
		}

		// TODO Make independent public and private fetch?
		privKey := []byte(input.Values.Get("istio.internal.remoteAuthnKeypair.priv").String())
		claims := map[string]string{
			"iss":   "d8-istio",
			"aud":   publicMetadata.ClusterUUID,
			"sub":   input.Values.Get("global.discovery.clusterUUID").String(),
			"scope": "private-multicluster",
		}
		bearerToken, err := jwt.GenerateJWT(privKey, claims, time.Minute)
		if err != nil {
			input.LogEntry.Warnf("can't generate auth token for endpoint %s of IstioMulticluster %s, error: %s", multiclusterInfo.PrivateMetadataEndpoint, multiclusterInfo.Name, err.Error())
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		bodyBytes, statusCode, err = lib.HTTPGet(dc.GetHTTPClient(), multiclusterInfo.PrivateMetadataEndpoint, bearerToken)
		if err != nil {
			input.LogEntry.Warnf("cannot fetch private metadata endpoint %s for IstioMulticluster %s, error: %s", multiclusterInfo.PrivateMetadataEndpoint, multiclusterInfo.Name, err.Error())
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		if statusCode != 200 {
			input.LogEntry.Warnf("cannot fetch private metadata endpoint %s for IstioMulticluster %s (HTTP Code %d)", multiclusterInfo.PrivateMetadataEndpoint, multiclusterInfo.Name, statusCode)
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		err = json.Unmarshal(bodyBytes, &privateMetadata)
		if err != nil {
			input.LogEntry.Warnf("cannot unmarshal private metadata endpoint %s for IstioMulticluster %s, error: %s", multiclusterInfo.PrivateMetadataEndpoint, multiclusterInfo.Name, err.Error())
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		if privateMetadata.NetworkName == "" || privateMetadata.APIHost == "" || privateMetadata.IngressGateways == nil {
			input.LogEntry.Warnf("bad private metadata format in endpoint %s for IstioMulticluster %s", multiclusterInfo.PrivateMetadataEndpoint, multiclusterInfo.Name)
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 0)
		err = multiclusterInfo.PatchMetadataCache(input.PatchCollector, "private", privateMetadata)
		if err != nil {
			return err
		}
	}
	return nil
}
