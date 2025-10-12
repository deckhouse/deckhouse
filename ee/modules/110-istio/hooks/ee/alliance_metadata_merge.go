/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	eeCrd "github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/ee/lib/crd"
	"github.com/deckhouse/deckhouse/go_lib/jwt"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type IstioFederationMergeCrdInfo struct {
	Name             string                             `json:"name"`
	TrustDomain      string                             `json:"trustDomain"`
	SpiffeEndpoint   string                             `json:"spiffeEndpoint"`
	IngressGateways  *[]eeCrd.FederationIngressGateways `json:"ingressGateways"`
	MetadataCA       string                             `json:"ca"`
	MetadataInsecure bool                               `json:"insecureSkipVerify"`
	PublicServices   *[]eeCrd.FederationPublicServices  `json:"publicServices"`
	Public           *eeCrd.AlliancePublicMetadata      `json:"public,omitempty"`
}

type IstioMulticlusterMergeCrdInfo struct {
	Name                 string                               `json:"name"`
	SpiffeEndpoint       string                               `json:"spiffeEndpoint"`
	EnableIngressGateway bool                                 `json:"enableIngressGateway"`
	MetadataCA           string                               `json:"ca"`
	MetadataInsecure     bool                                 `json:"insecureSkipVerify"`
	APIHost              string                               `json:"apiHost"`
	NetworkName          string                               `json:"networkName"`
	APIJWT               string                               `json:"apiJWT"`
	IngressGateways      *[]eeCrd.MulticlusterIngressGateways `json:"ingressGateways"`
	Public               *eeCrd.AlliancePublicMetadata        `json:"public,omitempty"`
}

func applyFederationMergeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var federation eeCrd.IstioFederation
	err := sdk.FromUnstructured(obj, &federation)
	if err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	me := federation.Spec.MetadataEndpoint
	me = strings.TrimSuffix(me, "/")

	var igs *[]eeCrd.FederationIngressGateways
	var pss *[]eeCrd.FederationPublicServices
	var p *eeCrd.AlliancePublicMetadata

	if federation.Status.MetadataCache.Private != nil {
		if federation.Status.MetadataCache.Private.IngressGateways != nil {
			igs = federation.Status.MetadataCache.Private.IngressGateways
		}
		if federation.Status.MetadataCache.Private.PublicServices != nil {
			pss = federation.Status.MetadataCache.Private.PublicServices
		}
	}
	if federation.Status.MetadataCache.Public != nil {
		p = federation.Status.MetadataCache.Public
	}

	return IstioFederationMergeCrdInfo{
		Name:             federation.GetName(),
		TrustDomain:      federation.Spec.TrustDomain,
		SpiffeEndpoint:   me + "/public/spiffe-bundle-endpoint",
		IngressGateways:  igs,
		MetadataCA:       federation.Spec.Metadata.ClusterCA,
		MetadataInsecure: federation.Spec.Metadata.EnableInsecureConnection,
		PublicServices:   pss,
		Public:           p,
	}, nil
}

func applyMulticlusterMergeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var multicluster eeCrd.IstioMulticluster

	err := sdk.FromUnstructured(obj, &multicluster)
	if err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	me := multicluster.Spec.MetadataEndpoint
	me = strings.TrimSuffix(me, "/")

	var igs *[]eeCrd.MulticlusterIngressGateways
	var apiHost string
	var networkName string
	var p *eeCrd.AlliancePublicMetadata

	if multicluster.Status.MetadataCache.Private != nil {
		if multicluster.Status.MetadataCache.Private.IngressGateways != nil {
			igs = multicluster.Status.MetadataCache.Private.IngressGateways
		}
		apiHost = multicluster.Status.MetadataCache.Private.APIHost
		networkName = multicluster.Status.MetadataCache.Private.NetworkName
	}
	if multicluster.Status.MetadataCache.Public != nil {
		p = multicluster.Status.MetadataCache.Public
	}

	return IstioMulticlusterMergeCrdInfo{
		Name:                 multicluster.GetName(),
		SpiffeEndpoint:       me + "/public/spiffe-bundle-endpoint",
		MetadataCA:           multicluster.Spec.Metadata.ClusterCA,
		MetadataInsecure:     multicluster.Spec.Metadata.EnableInsecureConnection,
		EnableIngressGateway: multicluster.Spec.EnableIngressGateway,
		APIHost:              apiHost,
		NetworkName:          networkName,
		IngressGateways:      igs,
		Public:               p,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: lib.Queue("alliance"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "federations",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "IstioFederation",
			FilterFunc: applyFederationMergeFilter,
		},
		{
			Name:       "multiclusters",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "IstioMulticlusters",
			FilterFunc: applyMulticlusterMergeFilter,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		// until the bug won't be solved https://github.com/istio/istio/issues/37925
		// {Name: "cron", Crontab: "0 3 * * *"}, // once a day to refresh apiJWT
		{Name: "cron", Crontab: "0 3 1 * *"}, // once a month to refresh apiJWT
	},
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, metadataMerge)

func metadataMerge(_ context.Context, input *go_hook.HookInput) error {
	var properFederations = make([]IstioFederationMergeCrdInfo, 0)
	var properMulticlusters = make([]IstioMulticlusterMergeCrdInfo, 0)
	var multiclustersNeedIngressGateway = false
	//                              map[clusterUUID]public
	var remotePublicMetadata = make(map[string]eeCrd.AlliancePublicMetadata)

	var myTrustDomain = input.Values.Get("global.discovery.clusterDomain").String()

federationsLoop:
	for federationInfo, err := range sdkobjectpatch.SnapshotIter[IstioFederationMergeCrdInfo](input.Snapshots.Get("federations")) {
		if err != nil {
			return fmt.Errorf("cannot iterate over federations: %w", err)
		}

		if federationInfo.TrustDomain == myTrustDomain {
			input.Logger.Warn("skipping IstioFederation with trustDomain equals to ours", slog.String("name", federationInfo.Name), slog.String("trust_domain", federationInfo.TrustDomain))
			continue federationsLoop
		}
		if federationInfo.Public == nil {
			input.Logger.Warn("public metadata for IstioFederation wasn't fetched yet", slog.String("name", federationInfo.Name))
			continue federationsLoop
		}

		remotePublicMetadata[federationInfo.Public.ClusterUUID] = *federationInfo.Public

		if federationInfo.PublicServices == nil {
			input.Logger.Warn("private metadata for IstioFederation wasn't fetched yet", slog.String("name", federationInfo.Name))
			continue
		}

		if federationInfo.IngressGateways == nil || len(*federationInfo.IngressGateways) == 0 {
			input.Logger.Warn("private metadata for IstioFederation wasn't fetched yet", slog.String("name", federationInfo.Name))
			continue federationsLoop
		}

		federationInfo.Public = nil
		properFederations = append(properFederations, federationInfo)
	}

multiclustersLoop:
	for multiclusterInfo, err := range sdkobjectpatch.SnapshotIter[IstioMulticlusterMergeCrdInfo](input.Snapshots.Get("multiclusters")) {
		if err != nil {
			return fmt.Errorf("cannot iterate over multiclusters: %w", err)
		}

		if multiclusterInfo.EnableIngressGateway {
			multiclustersNeedIngressGateway = true
		}

		if multiclusterInfo.Public == nil {
			input.Logger.Warn("public metadata for IstioMulticluster wasn't fetched yet", slog.String("name", multiclusterInfo.Name))
			continue multiclustersLoop
		}

		remotePublicMetadata[multiclusterInfo.Public.ClusterUUID] = *multiclusterInfo.Public

		if multiclusterInfo.APIHost == "" || multiclusterInfo.NetworkName == "" {
			input.Logger.Warn("private metadata for IstioMulticluster wasn't fetched yet", slog.String("name", multiclusterInfo.Name))
			continue multiclustersLoop
		}
		if multiclusterInfo.EnableIngressGateway &&
			(multiclusterInfo.IngressGateways == nil || len(*multiclusterInfo.IngressGateways) == 0) {
			input.Logger.Warn("ingressGateways for IstioMulticluster weren't fetched yet", slog.String("name", multiclusterInfo.Name))
			continue multiclustersLoop
		}

		privKey := []byte(input.Values.Get("istio.internal.remoteAuthnKeypair.priv").String())
		claims := map[string]string{
			"iss":   "d8-istio",
			"aud":   multiclusterInfo.Public.ClusterUUID,
			"sub":   input.Values.Get("global.discovery.clusterUUID").String(),
			"scope": "api",
		}
		// until the bug won't be solved https://github.com/istio/istio/issues/37925
		// multiclusterInfo.APIJWT, err = jwt.GenerateJWT(privKey, claims, time.Hour*25)
		multiclusterInfo.APIJWT, err = jwt.GenerateJWT(privKey, claims, time.Hour*24*366)
		if err != nil {
			input.Logger.Warn("can't generate auth token for remote api of IstioMulticluster", slog.String("name", multiclusterInfo.Name), log.Err(err))
			continue multiclustersLoop
		}

		multiclusterInfo.Public = nil
		properMulticlusters = append(properMulticlusters, multiclusterInfo)
	}

	input.Values.Set("istio.internal.federations", properFederations)
	input.Values.Set("istio.internal.multiclusters", properMulticlusters)
	input.Values.Set("istio.internal.multiclustersNeedIngressGateway", multiclustersNeedIngressGateway)
	input.Values.Set("istio.internal.remotePublicMetadata", remotePublicMetadata)

	return nil
}
