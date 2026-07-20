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
	"strconv"
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
	federationMetricsGroup = "federation_discovery"
	federationMetricName   = "d8_istio_federation_metadata_endpoints_fetch_error_count"
)

type IstioFederationDiscoveryCrdInfo struct {
	Name                     string
	ClusterUUID              string
	TrustDomain              string
	MetadataExporterCA       string
	EnableInsecureConnection bool
	PublicMetadataEndpoint   string
	PrivateMetadataEndpoint  string
	PriorConditions          []metav1.Condition
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
				scope: meta,
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
		MetadataExporterCA:       federation.Spec.Metadata.CA,
		EnableInsecureConnection: federation.Spec.Metadata.EnableInsecureConnection,
		ClusterUUID:              clusterUUID,
		PublicMetadataEndpoint:   me + "/public/public.json",
		PrivateMetadataEndpoint:  me + "/private/federation.json",
		PriorConditions:          federation.Status.Conditions,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: lib.Queue("federation"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "federations",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "IstioFederation",
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
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
		prior := priorAllianceConditionsByType(federationInfo.PriorConditions)

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

		if federationInfo.MetadataExporterCA != "" {
			caCerts := [][]byte{[]byte(federationInfo.MetadataExporterCA)}
			httpOption = append(httpOption, http.WithAdditionalCACerts(caCerts))
		} else if federationInfo.EnableInsecureConnection {
			httpOption = append(httpOption, http.WithInsecureSkipVerify())
		}

		bodyBytes, statusCode, err := lib.HTTPGet(dc.GetHTTPClient(httpOption...), federationInfo.PublicMetadataEndpoint, "")
		if err != nil {
			t := timeNow()
			input.Logger.Warn("cannot fetch public metadata endpoint for IstioFederation", slog.String("endpoint", federationInfo.PublicMetadataEndpoint), slog.String("name", federationInfo.Name), log.Err(err))
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PublicMetadataEndpoint, 1)
			publicCondition := getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionFalse, "FetchFailed", err.Error(), t)
			pendingPrivateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionUnknown, "AwaitingPublic", "Public metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioFederation", federationInfo.Name, federationInfo.PriorConditions, []metav1.Condition{publicCondition, pendingPrivateCondition}, t)
			continue
		}
		if statusCode != 200 {
			t := timeNow()
			input.Logger.Warn("cannot fetch public metadata endpoint for IstioFederation", slog.String("endpoint", federationInfo.PublicMetadataEndpoint), slog.String("name", federationInfo.Name), slog.Int("http_code", statusCode))
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PublicMetadataEndpoint, 1)
			msg := fmt.Sprintf("HTTP status %d when fetching public metadata", statusCode)
			publicCondition := getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionFalse, "NonOKResponse", msg, t)
			pendingPrivateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionUnknown, "AwaitingPublic", "Public metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioFederation", federationInfo.Name, federationInfo.PriorConditions, []metav1.Condition{publicCondition, pendingPrivateCondition}, t)
			continue
		}
		err = json.Unmarshal(bodyBytes, &publicMetadata)
		if err != nil {
			t := timeNow()
			input.Logger.Warn("cannot unmarshal public metadata endpoint for IstioFederation", slog.String("endpoint", federationInfo.PublicMetadataEndpoint), slog.String("name", federationInfo.Name), log.Err(err))
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PublicMetadataEndpoint, 1)
			publicCondition := getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionFalse, "InvalidJSON", err.Error(), t)
			pendingPrivateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionUnknown, "AwaitingPublic", "Public metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioFederation", federationInfo.Name, federationInfo.PriorConditions, []metav1.Condition{publicCondition, pendingPrivateCondition}, t)
			continue
		}
		if publicMetadata.ClusterUUID == "" || publicMetadata.AuthnKeyPub == "" || publicMetadata.RootCA == "" {
			t := timeNow()
			input.Logger.Warn("bad public metadata format in endpoint for IstioFederation", slog.String("endpoint", federationInfo.PublicMetadataEndpoint), slog.String("name", federationInfo.Name))
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PublicMetadataEndpoint, 1)
			publicCondition := getCondition(AllianceConditionPublicMetadataExchangeReady, prior, metav1.ConditionFalse, "InvalidPublicMetadata", "clusterUUID, authnKeyPub, and rootCA must be non-empty", t)
			pendingPrivateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionUnknown, "AwaitingPublic", "Public metadata exchange has not succeeded yet.", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioFederation", federationInfo.Name, federationInfo.PriorConditions, []metav1.Condition{publicCondition, pendingPrivateCondition}, t)
			continue
		}

		federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PublicMetadataEndpoint, 0)
		err = federationInfo.PatchMetadataCache(input.PatchCollector, "public", publicMetadata)
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
			"scope": "private-federation",
		}
		bearerToken, err := jwt.GenerateJWT(privKey, claims, time.Minute)
		if err != nil {
			t := timeNow()
			input.Logger.Warn("can't generate auth token for endpoint of IstioFederation", slog.String("endpoint", federationInfo.PrivateMetadataEndpoint), slog.String("name", federationInfo.Name), log.Err(err))
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			privateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionFalse, "TokenGenerationFailed", err.Error(), t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioFederation", federationInfo.Name, federationInfo.PriorConditions, []metav1.Condition{publicCondition, privateCondition}, t)
			continue
		}
		bodyBytes, statusCode, err = lib.HTTPGet(dc.GetHTTPClient(httpOption...), federationInfo.PrivateMetadataEndpoint, bearerToken)
		if err != nil {
			t := timeNow()
			input.Logger.Warn("cannot fetch private metadata endpoint for IstioFederation", slog.String("endpoint", federationInfo.PrivateMetadataEndpoint), slog.String("name", federationInfo.Name), log.Err(err))
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			privateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionFalse, "FetchFailed", err.Error(), t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioFederation", federationInfo.Name, federationInfo.PriorConditions, []metav1.Condition{publicCondition, privateCondition}, t)
			continue
		}
		if statusCode != 200 {
			t := timeNow()
			input.Logger.Warn("cannot fetch private metadata endpoint for IstioFederation", slog.String("endpoint", federationInfo.PrivateMetadataEndpoint), slog.String("name", federationInfo.Name), slog.Int("http_code", statusCode))
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			msg := fmt.Sprintf("HTTP status %d when fetching private metadata", statusCode)
			privateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionFalse, "NonOKResponse", msg, t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioFederation", federationInfo.Name, federationInfo.PriorConditions, []metav1.Condition{publicCondition, privateCondition}, t)
			continue
		}
		err = json.Unmarshal(bodyBytes, &privateMetadata)
		if err != nil {
			t := timeNow()
			input.Logger.Warn("cannot unmarshal private metadata endpoint for IstioFederation", slog.String("endpoint", federationInfo.PrivateMetadataEndpoint), slog.String("name", federationInfo.Name), log.Err(err))
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			privateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionFalse, "InvalidJSON", err.Error(), t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioFederation", federationInfo.Name, federationInfo.PriorConditions, []metav1.Condition{publicCondition, privateCondition}, t)
			continue
		}
		if privateMetadata.IngressGateways == nil || privateMetadata.PublicServices == nil {
			t := timeNow()
			input.Logger.Warn("bad private metadata format in endpoint for IstioFederation", slog.String("endpoint", federationInfo.PrivateMetadataEndpoint), slog.String("name", federationInfo.Name))
			federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 1)
			privateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionFalse, "InvalidPrivateMetadata", "ingressGateways and publicServices must be set", t)
			patchAllianceDiscoveryConditions(input.PatchCollector, "IstioFederation", federationInfo.Name, federationInfo.PriorConditions, []metav1.Condition{publicCondition, privateCondition}, t)
			continue
		}

		updatePortProtocols(privateMetadata.PublicServices, defaultProtocol, protocolMap)
		federationInfo.SetMetricMetadataEndpointError(input.MetricsCollector, federationInfo.PrivateMetadataEndpoint, 0)
		err = federationInfo.PatchMetadataCache(input.PatchCollector, "private", privateMetadata)
		if err != nil {
			return err
		}

		tDone := timeNow()
		privateCondition := getCondition(AllianceConditionPrivateMetadataExchangeReady, prior, metav1.ConditionTrue, "Succeeded", "Private metadata exchange succeeded.", tDone)
		patchAllianceDiscoveryConditions(input.PatchCollector, "IstioFederation", federationInfo.Name, federationInfo.PriorConditions, []metav1.Condition{publicCondition, privateCondition}, tDone)

		var countServices = 0
		if privateMetadata.PublicServices != nil {
			countServices = len(*privateMetadata.PublicServices)
		}
		input.Logger.Info(fmt.Sprintf("Cluster name: %s connected successfully, published services: %s", myTrustDomain, strconv.Itoa(countServices)))
	}
	return nil
}

func updatePortProtocols(services *[]eeCrd.FederationPublicService, defaultProtocol string, protocolMap map[string]string) {
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

const (
	AllianceConditionPublicMetadataExchangeReady  = "PublicMetadataExchangeReady"
	AllianceConditionPrivateMetadataExchangeReady = "PrivateMetadataExchangeReady"
	AllianceConditionRemoteAPIServerReady         = "RemoteAPIServerReady"
)

func timeNow() metav1.Time {
	return metav1.NewTime(time.Now().UTC())
}

func getCondition(condType string, prior map[string]metav1.Condition, status metav1.ConditionStatus, reason, message string, probe metav1.Time) metav1.Condition {
	if reason == "" {
		reason = "Unknown"
	}
	transition := probe
	if p, ok := prior[condType]; ok && p.Status == status && p.Reason == reason && p.Message == message {
		transition = p.LastTransitionTime
	}
	return metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: transition,
	}
}

func priorAllianceConditionsByType(prior []metav1.Condition) map[string]metav1.Condition {
	out := make(map[string]metav1.Condition, len(prior))
	for i := range prior {
		c := prior[i]
		out[c.Type] = c
	}
	return out
}

// mergeDiscoveryConditionsPreserveUnmanaged keeps status conditions not owned by discovery hooks
func mergeDiscoveryConditionsPreserveUnmanaged(prior []metav1.Condition, incoming []metav1.Condition) []metav1.Condition {
	incomingTypes := make(map[string]struct{}, len(incoming))
	for i := range incoming {
		incomingTypes[incoming[i].Type] = struct{}{}
	}
	var tail []metav1.Condition
	for i := range prior {
		if _, replaced := incomingTypes[prior[i].Type]; replaced {
			continue
		}
		tail = append(tail, prior[i])
	}
	out := make([]metav1.Condition, 0, len(incoming)+len(tail))
	out = append(out, incoming...)
	out = append(out, tail...)
	return out
}

func allianceDiscoveryConditionsToSlice(conds []metav1.Condition, probe metav1.Time) []interface{} {
	out := make([]interface{}, 0, len(conds))
	probeTime := probe.Time.UTC().Format(time.RFC3339)
	for i := range conds {
		c := conds[i]
		out = append(out, map[string]interface{}{
			"type":               c.Type,
			"status":             string(c.Status),
			"reason":             c.Reason,
			"message":            c.Message,
			"lastTransitionTime": c.LastTransitionTime.Time.UTC().Format(time.RFC3339),
			"lastProbeTime":      probeTime,
		})
	}
	return out
}

func patchAllianceDiscoveryConditions(pc go_hook.PatchCollector, crdKind, name string, prior []metav1.Condition, conditions []metav1.Condition, probe metav1.Time) {
	merged := mergeDiscoveryConditionsPreserveUnmanaged(prior, conditions)
	pc.PatchWithMerge(
		map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": allianceDiscoveryConditionsToSlice(merged, probe),
			},
		},
		"deckhouse.io/v1alpha1",
		crdKind,
		"",
		name,
		object_patch.WithSubresource("/status"),
	)
}
