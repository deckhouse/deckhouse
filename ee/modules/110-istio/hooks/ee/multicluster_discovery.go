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
			pub := getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionFalse, "FetchFailed", err.Error(), t)
			priv := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionUnknown, "AwaitingPublic", "Public metadata exchange has not succeeded yet.", t)
			api := getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{pub, priv, api}, t)
			continue
		}
		if statusCode != 200 {
			t := timeNow()
			input.Logger.Warn("cannot fetch public metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PublicMetadataEndpoint), slog.String("name", multiclusterInfo.Name), slog.Int("http_code", statusCode))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			msg := fmt.Sprintf("HTTP status %d when fetching public metadata", statusCode)
			pub := getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionFalse, "NonOKResponse", msg, t)
			priv := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionUnknown, "AwaitingPublic", "Public metadata exchange has not succeeded yet.", t)
			api := getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{pub, priv, api}, t)
			continue
		}
		err = json.Unmarshal(bodyBytes, &publicMetadata)
		if err != nil {
			t := timeNow()
			input.Logger.Warn("cannot unmarshal public metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PublicMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			pub := getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionFalse, "InvalidJSON", err.Error(), t)
			priv := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionUnknown, "AwaitingPublic", "Public metadata exchange has not succeeded yet.", t)
			api := getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{pub, priv, api}, t)
			continue
		}
		if publicMetadata.ClusterUUID == "" || publicMetadata.AuthnKeyPub == "" || publicMetadata.RootCA == "" {
			t := timeNow()
			input.Logger.Warn("bad public metadata format in endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PublicMetadataEndpoint), slog.String("name", multiclusterInfo.Name))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 1)
			pub := getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionFalse, "InvalidPublicMetadata", "clusterUUID, authnKeyPub, and rootCA must be non-empty", t)
			priv := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionUnknown, "AwaitingPublic", "Public metadata exchange has not succeeded yet.", t)
			api := getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{pub, priv, api}, t)
			continue
		}
		multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PublicMetadataEndpoint, 0)
		tPub := timeNow()
		pubOK := getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionTrue, "Succeeded", "Public metadata exchange succeeded.", tPub)
		privPending := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange not evaluated yet.", tPub)
		remotePending := getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", tPub)
		err = multiclusterInfo.PatchMetadataCache(input.PatchCollector, "public", publicMetadata)
		if err != nil {
			return err
		}
		patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{pubOK, privPending, remotePending}, tPub)

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
			privFail := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionFalse, "TokenGenerationFailed", err.Error(), t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionTrue, pubOK.Reason, pubOK.Message, t), privFail, getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)}, t)
			continue
		}
		bodyBytes, statusCode, err = lib.HTTPGet(dc.GetHTTPClient(httpOption...), multiclusterInfo.PrivateMetadataEndpoint, bearerToken)
		if err != nil {
			t := timeNow()
			input.Logger.Warn("cannot fetch private metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			privFail := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionFalse, "FetchFailed", err.Error(), t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionTrue, pubOK.Reason, pubOK.Message, t), privFail, getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)}, t)
			continue
		}
		if statusCode != 200 {
			t := timeNow()
			input.Logger.Warn("cannot fetch private metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name), slog.Int("http_code", statusCode))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			msg := fmt.Sprintf("HTTP status %d when fetching private metadata", statusCode)
			privFail := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionFalse, "NonOKResponse", msg, t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionTrue, pubOK.Reason, pubOK.Message, t), privFail, getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)}, t)
			continue
		}
		err = json.Unmarshal(bodyBytes, &privateMetadata)
		if err != nil {
			t := timeNow()
			input.Logger.Warn("cannot unmarshal private metadata endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name), log.Err(err))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			privFail := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionFalse, "InvalidJSON", err.Error(), t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionTrue, pubOK.Reason, pubOK.Message, t), privFail, getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)}, t)
			continue
		}
		if privateMetadata.NetworkName == "" || privateMetadata.APIHost == "" || privateMetadata.IngressGateways == nil {
			t := timeNow()
			input.Logger.Warn("bad private metadata format in endpoint for IstioMulticluster", slog.String("endpoint", multiclusterInfo.PrivateMetadataEndpoint), slog.String("name", multiclusterInfo.Name))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			privFail := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionFalse, "InvalidPrivateMetadata", "networkName, apiHost must be non-empty and ingressGateways must be set", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionTrue, pubOK.Reason, pubOK.Message, t), privFail, getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)}, t)
			continue
		}
		if multiclusterInfo.EnableIngressGateway && len(*privateMetadata.IngressGateways) == 0 {
			t := timeNow()
			input.Logger.Warn("ingressGateways for IstioMulticluster weren't fetched yet", slog.String("name", multiclusterInfo.Name))
			multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 1)
			privFail := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionFalse, "MissingIngressGateways", "enableIngressGateway is true but ingressGateways list is empty", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionTrue, pubOK.Reason, pubOK.Message, t), privFail, getCondition(AllianceConditionRemoteAPIServerReady, prior, metav1.ConditionUnknown, "AwaitingPrivate", "Private metadata exchange has not succeeded yet.", t)}, t)
			continue
		}
		multiclusterInfo.SetMetricMetadataEndpointError(input.MetricsCollector, multiclusterInfo.PrivateMetadataEndpoint, 0)
		tDone := timeNow()
		rs, rr, rm := checkMulticlusterRemoteAPIServer(dc.GetHTTPClient(httpOption...), privateMetadata.APIHost)
		remoteCond := getCondition(AllianceConditionRemoteAPIServerReady, prior, rs, rr, rm, tDone)
		privOK := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionTrue, "Succeeded", "Private metadata exchange succeeded.", tDone)
		pubCondDone := getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionTrue, pubOK.Reason, pubOK.Message, tDone)
		err = multiclusterInfo.PatchMetadataCache(input.PatchCollector, "private", privateMetadata)
		if err != nil {
			return err
		}
		patchAllianceDiscoveryConditions(input.PatchCollector, "IstioMulticluster", multiclusterInfo.Name, []metav1.Condition{pubCondDone, privOK, remoteCond}, tDone)
	}
	return nil
}

func checkMulticlusterRemoteAPIServer(client http.Client, apiHost string) (metav1.ConditionStatus, string, string) {
	host := strings.TrimSpace(apiHost)
	if host == "" {
		return metav1.ConditionFalse, "MissingAPIHost", "private metadata has empty apiHost"
	}
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}
	host = strings.TrimSuffix(host, "/")
	url := host + "/"
	body, code, err := lib.HTTPGet(client, url, "")
	if err != nil {
		_ = body
		return metav1.ConditionFalse, "RemoteAPIUnreachable", fmt.Sprintf("GET %s: %v", url, err)
	}
	_ = body
	if code >= 500 {
		return metav1.ConditionFalse, "RemoteAPIBadResponse", fmt.Sprintf("GET %s returned HTTP %d", url, code)
	}
	return metav1.ConditionTrue, "RemoteAPIReachable", fmt.Sprintf("GET %s returned HTTP %d", url, code)
}
