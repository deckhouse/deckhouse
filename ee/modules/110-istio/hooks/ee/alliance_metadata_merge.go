/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	eeCrd "github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/ee/lib/crd"
	"github.com/deckhouse/deckhouse/go_lib/jwt"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

type IstioFederationMergeCrdInfo struct {
	Name            string                             `json:"name"`
	TrustDomain     string                             `json:"trustDomain"`
	SpiffeEndpoint  string                             `json:"spiffeEndpoint"`
	IngressGateways *[]eeCrd.FederationIngressGateways `json:"ingressGateways"`
	PublicServices  *[]eeCrd.FederationPublicServices  `json:"publicServices"`
	Public          *eeCrd.AlliancePublicMetadata      `json:"public,omitempty"`
}

type IstioMulticlusterMergeCrdInfo struct {
	Name                 string                               `json:"name"`
	SpiffeEndpoint       string                               `json:"spiffeEndpoint"`
	EnableIngressGateway bool                                 `json:"enableIngressGateway"`
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
		return nil, err
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
		Name:            federation.GetName(),
		TrustDomain:     federation.Spec.TrustDomain,
		SpiffeEndpoint:  me + "/public/spiffe-bundle-endpoint",
		IngressGateways: igs,
		PublicServices:  pss,
		Public:          p,
	}, nil
}

func applyMulticlusterMergeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var multicluster eeCrd.IstioMulticluster

	err := sdk.FromUnstructured(obj, &multicluster)
	if err != nil {
		return nil, err
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

func metadataMerge(input *go_hook.HookInput) error {
	var err error
	var properFederations = make([]IstioFederationMergeCrdInfo, 0)
	var properMulticlusters = make([]IstioMulticlusterMergeCrdInfo, 0)
	var multiclustersNeedIngressGateway = false
	//                              map[clusterUUID]public
	var remotePublicMetadata = make(map[string]eeCrd.AlliancePublicMetadata)

	var myTrustDomain = input.Values.Get("global.discovery.clusterDomain").String()

federationsLoop:
	for _, federation := range input.Snapshots["federations"] {
		federationInfo := federation.(IstioFederationMergeCrdInfo)
		if federationInfo.TrustDomain == myTrustDomain {
			input.LogEntry.Warnf("skipping IstioFederation %s with trustDomain equals to ours: %s", federationInfo.Name, federationInfo.TrustDomain)
			continue federationsLoop
		}
		if federationInfo.Public == nil {
			input.LogEntry.Warnf("public metadata for IstioFederation %s wasn't fetched yet", federationInfo.Name)
			continue federationsLoop
		}

		remotePublicMetadata[federationInfo.Public.ClusterUUID] = *federationInfo.Public

		if federationInfo.PublicServices == nil {
			input.LogEntry.Warnf("private metadata for IstioFederation %s wasn't fetched yet", federationInfo.Name)
			continue
		}
		for _, ps := range *federationInfo.PublicServices {
			if ps.VirtualIP == "" {
				input.LogEntry.Warnf("virtualIP wasn't set for publicService %s of IstioFederation %s", ps.Hostname, federationInfo.Name)
				continue federationsLoop
			}
		}

		if federationInfo.IngressGateways == nil || len(*federationInfo.IngressGateways) == 0 {
			input.LogEntry.Warnf("private metadata for IstioFederation %s wasn't fetched yet", federationInfo.Name)
			continue federationsLoop
		}

		federationInfo.Public = nil
		properFederations = append(properFederations, federationInfo)
	}

multiclustersLoop:
	for _, multicluster := range input.Snapshots["multiclusters"] {
		multiclusterInfo := multicluster.(IstioMulticlusterMergeCrdInfo)

		if multiclusterInfo.EnableIngressGateway {
			multiclustersNeedIngressGateway = true
		}

		if multiclusterInfo.Public == nil {
			input.LogEntry.Warnf("public metadata for IstioMulticluster %s wasn't fetched yet", multiclusterInfo.Name)
			continue multiclustersLoop
		}

		remotePublicMetadata[multiclusterInfo.Public.ClusterUUID] = *multiclusterInfo.Public

		if multiclusterInfo.APIHost == "" || multiclusterInfo.NetworkName == "" {
			input.LogEntry.Warnf("private metadata for IstioMulticluster %s wasn't fetched yet", multiclusterInfo.Name)
			continue multiclustersLoop
		}
		if multiclusterInfo.EnableIngressGateway &&
			(multiclusterInfo.IngressGateways == nil || len(*multiclusterInfo.IngressGateways) == 0) {
			input.LogEntry.Warnf("ingressGateways for IstioMulticluster %s weren't fetched yet", multiclusterInfo.Name)
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
			input.LogEntry.Warnf("can't generate auth token for remote api of IstioMulticluster %s, error: %s", multiclusterInfo.Name, err.Error())
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
