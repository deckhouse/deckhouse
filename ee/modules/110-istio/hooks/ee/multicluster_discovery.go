/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
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
	multiclusterMetricsGroup = "multicluster_discovery"
	multiclusterMetricName   = "d8_istio_multicluster_metadata_endpoints_fetch_error_count"
)

type IstioMulticlusterDiscoveryCrdInfo struct {
	Name                     string
	ClusterUUID              string
	EnableIngressGateway     bool
	MetadataExporterCA       string
	EnableInsecureConnection bool
	PublicMetadataEndpoint   string
	PrivateMetadataEndpoint  string
}

func (i *IstioMulticlusterDiscoveryCrdInfo) SetMetricMetadataEndpointError(mc sdkpkg.MetricsCollector, endpoint string, isError float64) {
	labels := map[string]string{
		"multicluster_name": i.Name,
		"endpoint":          endpoint,
	}

	mc.Set(multiclusterMetricName, isError, labels, metrics.WithGroup(multiclusterMetricsGroup))
}

func (i *IstioMulticlusterDiscoveryCrdInfo) PatchMetadataCache(pc go_hook.PatchCollector, scope string, meta interface{}) error {
	patch := map[string]interface{}{
		"status": map[string]interface{}{
			"metadataCache": map[string]interface{}{
				scope:                        meta,
				scope + "LastFetchTimestamp": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	pc.PatchWithMerge(patch, "deckhouse.io/v1alpha1", "IstioMulticluster", "", i.Name, object_patch.WithSubresource("/status"))
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
		Name:                     multicluster.GetName(),
		EnableIngressGateway:     multicluster.Spec.EnableIngressGateway,
		MetadataExporterCA:       multicluster.Spec.Metadata.CA,
		EnableInsecureConnection: multicluster.Spec.Metadata.EnableInsecureConnection,
		ClusterUUID:              clusterUUID,
		PublicMetadataEndpoint:   me + "/public/public.json",
		PrivateMetadataEndpoint:  me + "/private/multicluster.json",
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: lib.Queue("multicluster"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "multiclusters",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "IstioMulticluster",
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   applyMulticlusterFilter,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "* * * * *"},
	},
}, dependency.WithExternalDependencies(multiclusterDiscovery))

func multiclusterDiscovery(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	input.MetricsCollector.Expire(multiclusterMetricsGroup)

	if !input.Values.Get("istio.multicluster.enabled").Bool() {
		return nil
	}
	if !input.Values.Get("istio.internal.remoteAuthnKeypair.priv").Exists() {
		input.Logger.Warn("authn keypair for signing requests to remote metadata endpoints isn't generated yet, retry in 1min")
		return nil
	}

	for multiclusterInfo, err := range sdkobjectpatch.SnapshotIter[IstioMulticlusterDiscoveryCrdInfo](input.Snapshots.Get("multiclusters")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over multiclusters: %v", err)
		}

		var publicMetadata eeCrd.AlliancePublicMetadata
		var privateMetadata eeCrd.MulticlusterPrivateMetadata
		var httpOption []http.Option

		if multiclusterInfo.MetadataExporterCA != "" {
			caCerts := [][]byte{[]byte(multiclusterInfo.MetadataExporterCA)}
			httpOption = append(httpOption, http.WithAdditionalCACerts(caCerts))
		} else if multiclusterInfo.EnableInsecureConnection {
			httpOption = append(httpOption, http.WithInsecureSkipVerify())
		}

		bodyBytes, statusCode, err := lib.HTTPGet(dc.GetHTTPClient(httpOption...), multiclusterInfo.PublicMetadataEndpoint, "")
		if err != nil {
			input.Logger.Warn("cannot fetch public metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PublicMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			continue
		}
		if statusCode != 200 {
			input.Logger.Warn("cannot fetch public metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PublicMetadataEndpoint), slog.String("name", multiclusterInfo.Name), slog.Int("http_code", statusCode))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			continue
		}
		err = json.Unmarshal(bodyBytes, &publicMetadata)
		if err != nil {
			input.Logger.Warn("cannot unmarshal public metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PublicMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			continue
		}
		if publicMetadata.ClusterUUID == "" || publicMetadata.AuthnKeyPub == "" || publicMetadata.RootCA == "" {
			input.Logger.Warn("bad public metadata format in endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PublicMetadataEndpoint), slog.String("name", multiclusterInfo.Name))
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
			input.Logger.Warn("can't generate auth token for endpoint of IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			continue
		}

		multiclusterInfo.checkDeprecatedSubdomainIn(input)

		bodyBytes, statusCode, err = lib.HTTPGet(dc.GetHTTPClient(httpOption...), multiclusterInfo.PrivateMetadataEndpoint, bearerToken)
		if err != nil {
			input.Logger.Warn("cannot fetch private metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		if statusCode != 200 {
			input.Logger.Warn("cannot fetch private metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name), slog.Int("http_code", statusCode))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		err = json.Unmarshal(bodyBytes, &privateMetadata)
		if err != nil {
			input.Logger.Warn("cannot unmarshal private metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			continue
		}
		if privateMetadata.NetworkName == "" || privateMetadata.APIHost == "" || privateMetadata.IngressGateways == nil {
			input.Logger.Warn("bad private metadata format in endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name))
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

func (i *IstioMulticlusterDiscoveryCrdInfo) checkDeprecatedSubdomainIn(input *go_hook.HookInput) {
	raw := strings.TrimSpace(i.PublicMetadataEndpoint)
	if raw == "" {
		input.Logger.Warn(
			"failed to validate metadataEndpoint subdomain",
			slog.String("name", i.Name),
			log.Err(fmt.Errorf("empty metadataEndpoint")),
		)
		return
	}
	parseInput := raw
	if !strings.Contains(parseInput, "://") {
		parseInput = "https://" + parseInput
	}
	u, err := url.Parse(parseInput)
	if err != nil {
		input.Logger.Warn(
			"failed to validate metadataEndpoint subdomain",
			log.Err(err),
		)
		return
	}
	hostname := u.Hostname()
	if hostname == "" {
		input.Logger.Warn(
			"failed to validate metadataEndpoint subdomain",
			slog.String("name", i.Name),
			slog.String("endpoint", i.PublicMetadataEndpoint),
			slog.String("reason", "no host in metadataEndpoint"),
		)
		return
	}
	h := strings.TrimSpace(strings.ToLower(hostname))
	leftmost, hostnameWithoutSubdomain, _ := strings.Cut(strings.TrimSuffix(h, "."), ".")

	if leftmost != "istio" {
		return
	}

	labels := map[string]string{
		"alliance_kind": "IstioMulticluster",
		"name":          i.Name,
		"hostname":      hostnameWithoutSubdomain,
	}
	input.MetricsCollector.Set(deprecatedAllianceSubdomainMetricName, 1, labels, metrics.WithGroup(multiclusterMetricsGroup))
}
