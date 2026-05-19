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
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	PriorConditions          []metav1.Condition
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
				scope: meta,
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
		PriorConditions:          multicluster.Status.Conditions,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: lib.Queue("multicluster"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "multiclusters",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "IstioMulticluster",
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
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
		prior := priorAllianceConditionsByType(multiclusterInfo.PriorConditions)

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
			t := timeNow()
			input.Logger.Warn("cannot fetch public metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PublicMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			publicCondition := getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionFalse, "FetchFailed", err.Error(), t)
			pendingPrivateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionUnknown, "AwaitingPublic", "Public metadata exchange has not succeeded yet.", t)
			pendingAPICondition := getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{publicCondition, pendingPrivateCondition, pendingAPICondition}, t)
			continue
		}
		if statusCode != 200 {
			t := timeNow()
			input.Logger.Warn("cannot fetch public metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PublicMetadataEndpoint), slog.String("name", multiclusterInfo.Name), slog.Int("http_code", statusCode))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			msg := fmt.Sprintf("HTTP status %d when fetching public metadata", statusCode)
			publicCondition := getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionFalse, "NonOKResponse", msg, t)
			pendingPrivateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionUnknown, "AwaitingPublic", "Public metadata exchange has not succeeded yet.", t)
			pendingAPICondition := getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{publicCondition, pendingPrivateCondition, pendingAPICondition}, t)
			continue
		}
		err = json.Unmarshal(bodyBytes, &publicMetadata)
		if err != nil {
			t := timeNow()
			input.Logger.Warn("cannot unmarshal public metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PublicMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			publicCondition := getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionFalse, "InvalidJSON", err.Error(), t)
			pendingPrivateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionUnknown, "AwaitingPublic", "Public metadata exchange has not succeeded yet.", t)
			pendingAPICondition := getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{publicCondition, pendingPrivateCondition, pendingAPICondition}, t)
			continue
		}
		if publicMetadata.ClusterUUID == "" || publicMetadata.AuthnKeyPub == "" || publicMetadata.RootCA == "" {
			t := timeNow()
			input.Logger.Warn("bad public metadata format in endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PublicMetadataEndpoint), slog.String("name", multiclusterInfo.Name))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			publicCondition := getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionFalse, "InvalidPublicMetadata", "clusterUUID, authnKeyPub, and rootCA must be non-empty", t)
			pendingPrivateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionUnknown, "AwaitingPublic", "Public metadata exchange has not succeeded yet.", t)
			pendingAPICondition := getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{publicCondition, pendingPrivateCondition, pendingAPICondition}, t)
			continue
		}

		multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 0)
		err = multiclusterInfo.PatchMetadataCache(input.PatchCollector, "public", publicMetadata)
		if err != nil {
			return err
		}

		t := timeNow()
		publicCondition := getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionTrue, "Succeeded", "Public metadata exchange succeeded.", t)

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
			t := timeNow()
			input.Logger.Warn("can't generate auth token for endpoint of IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			privateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionFalse, "TokenGenerationFailed", err.Error(), t)
			pendingAPICondition := getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{publicCondition, privateCondition, pendingAPICondition}, t)
			continue
		}
		bodyBytes, statusCode, err = lib.HTTPGet(dc.GetHTTPClient(httpOption...), multiclusterInfo.PrivateMetadataEndpoint, bearerToken)
		if err != nil {
			t := timeNow()
			input.Logger.Warn("cannot fetch private metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			privateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionFalse, "FetchFailed", err.Error(), t)
			pendingAPICondition := getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{publicCondition, privateCondition, pendingAPICondition}, t)
			continue
		}
		if statusCode != 200 {
			t := timeNow()
			input.Logger.Warn("cannot fetch private metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name), slog.Int("http_code", statusCode))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			msg := fmt.Sprintf("HTTP status %d when fetching private metadata", statusCode)
			privateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionFalse, "NonOKResponse", msg, t)
			pendingAPICondition := getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{publicCondition, privateCondition, pendingAPICondition}, t)
			continue
		}
		err = json.Unmarshal(bodyBytes, &privateMetadata)
		if err != nil {
			t := timeNow()
			input.Logger.Warn("cannot unmarshal private metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			privateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionFalse, "InvalidJSON", err.Error(), t)
			pendingAPICondition := getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{publicCondition, privateCondition, pendingAPICondition}, t)
			continue
		}
		if privateMetadata.NetworkName == "" || privateMetadata.APIHost == "" || privateMetadata.IngressGateways == nil {
			t := timeNow()
			input.Logger.Warn("bad private metadata format in endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			privateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionFalse, "InvalidPrivateMetadata", "networkName, apiHost must be non-empty and ingressGateways must be set", t)
			pendingAPICondition := getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{publicCondition, privateCondition, pendingAPICondition}, t)
			continue
		}
		if multiclusterInfo.EnableIngressGateway && len(*privateMetadata.IngressGateways) == 0 {
			t := timeNow()
			input.Logger.Warn("ingressGateways for IstioMulticluster weren't fetched yet", slog.String("name", multiclusterInfo.Name))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			privateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionFalse, "MissingIngressGateways", "enableIngressGateway is true but ingressGateways list is empty", t)
			pendingAPICondition := getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{publicCondition, privateCondition, pendingAPICondition}, t)
			continue
		}

		multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 0)
		err = multiclusterInfo.PatchMetadataCache(input.PatchCollector, "private", privateMetadata)
		if err != nil {
			return err
		}
		tDone := timeNow()
		privateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionTrue, "Succeeded", "Private metadata exchange succeeded.", tDone)

		apiClaims := map[string]string{
			"iss":   "d8-istio",
			"aud":   publicMetadata.ClusterUUID,
			"sub":   input.Values.Get("global.discovery.clusterUUID").String(),
			"scope": "api",
		}
		apiJWT, err := jwt.GenerateJWT(privKey, apiClaims, time.Hour*24*366)
		if err != nil {
			tDone := timeNow()
			input.Logger.Warn("can't generate API-scope JWT for IstioMulticluster remote API check", slog.String("name", multiclusterInfo.Name), log.Err(err))
			apiCondition := getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionFalse, "TokenGenerationFailed", err.Error(), tDone)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{publicCondition, privateCondition, apiCondition}, tDone)
			continue
		}

		rs, rr, rm := checkMulticlusterRemoteAPIServer(dc.GetHTTPClient(httpOption...), privateMetadata.APIHost, apiJWT)
		tDone = timeNow()
		apiCondition := getCondition(AllianceConditionRemoteAPIServerReady, prior, rs, rr, rm, tDone)
		patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{publicCondition, privateCondition, apiCondition}, tDone)
	}
	return nil
}

type multiclusterRemoteAPIVersions struct {
	Kind     string   `json:"kind"`
	Versions []string `json:"versions"`
}

func multiclusterRemoteAPIProbeURL(apiHost string) (string, error) {
	h := strings.TrimSpace(apiHost)
	if h == "" {
		return "", fmt.Errorf("private metadata has empty apiHost")
	}
	if !strings.Contains(h, "://") {
		h = "https://" + h
	}
	h = strings.TrimSuffix(h, "/")
	return h + "/api", nil
}

func checkMulticlusterRemoteAPIServer(client http.Client, apiHost, bearerToken string) (metav1.ConditionStatus, string, string) {
	url, err := multiclusterRemoteAPIProbeURL(apiHost)
	if err != nil {
		return metav1.ConditionFalse, "MissingAPIHost", err.Error()
	}
	body, code, err := lib.HTTPGet(client, url, bearerToken)
	if err != nil {
		return metav1.ConditionFalse, "RemoteAPIUnreachable", fmt.Sprintf("GET %s: %v", url, err)
	}
	if code != 200 {
		return metav1.ConditionFalse, "RemoteAPIBadResponse", fmt.Sprintf("GET %s returned HTTP %d", url, code)
	}
	var parsed multiclusterRemoteAPIVersions
	if err := json.Unmarshal(body, &parsed); err != nil {
		return metav1.ConditionFalse, "RemoteAPIInvalidResponse", fmt.Sprintf("GET %s: invalid JSON: %v", url, err)
	}
	if parsed.Kind != "APIVersions" {
		return metav1.ConditionFalse, "RemoteAPIUnexpectedResponse", fmt.Sprintf("GET %s: expected kind APIVersions, got %q", url, parsed.Kind)
	}
	return metav1.ConditionTrue, "RemoteAPIReachable", fmt.Sprintf("GET %s returned HTTP %d with kind APIVersions", url, code)
}
