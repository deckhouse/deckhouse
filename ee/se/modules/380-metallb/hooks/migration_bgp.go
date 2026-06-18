/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// TODO: remove after 1.82.

package hooks

import (
	"context"
	"fmt"
	"regexp"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

var communityPattern = regexp.MustCompile(`^([0-9]+:[0-9]+|no-export|no-advertise|local-as|none)$`)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/metallb/migration_bgp",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "module_config",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"metallb"},
			},
			FilterFunc: filterBGPModuleConfig,
		},
		{
			Name:                "existing_pools",
			ApiVersion:          "network.deckhouse.io/v1alpha1",
			Kind:                "MetalLoadBalancerPool",
			FilterFunc:          filterName,
			ExecuteHookOnEvents: ptr.To(false),
		},
		{
			Name:                "existing_peers",
			ApiVersion:          "network.deckhouse.io/v1alpha1",
			Kind:                "MetalLoadBalancerBGPPeer",
			FilterFunc:          filterName,
			ExecuteHookOnEvents: ptr.To(false),
		},
		{
			Name:                "existing_configs",
			ApiVersion:          "network.deckhouse.io/v1alpha1",
			Kind:                "MetalLoadBalancerConfiguration",
			FilterFunc:          filterName,
			ExecuteHookOnEvents: ptr.To(false),
		},
	},
}, migrateBGP)

func filterName(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

type legacyBGPConfig struct {
	Version        int
	BGPPeers       []legacyBGPPeer
	AddressPools   []legacyAddressPool
	BGPCommunities map[string]string
	Speaker        legacySpeaker
}

type legacyAddressPool struct {
	Name              string                   `json:"name"`
	Protocol          string                   `json:"protocol"`
	Addresses         []string                 `json:"addresses"`
	BGPAdvertisements []legacyBGPAdvertisement `json:"bgp-advertisements"`
}

type legacyBGPAdvertisement struct {
	Communities       []string `json:"communities"`
	LocalPref         *int     `json:"localpref"`
	AggregationLength *int     `json:"aggregation-length"`
}

type legacyBGPPeer struct {
	PeerAddress string  `json:"peer-address"`
	PeerASN     float64 `json:"peer-asn"`
	MyASN       float64 `json:"my-asn"`
	RouterID    string  `json:"router-id"`
	PeerPort    *int    `json:"peer-port"`
	HoldTime    string  `json:"hold-time"`
}

type legacySpeaker struct {
	NodeSelector map[string]string `json:"nodeSelector"`
}

func filterBGPModuleConfig(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var mc struct {
		Spec struct {
			Version  int `json:"version"`
			Settings struct {
				BGPPeers       []legacyBGPPeer     `json:"bgpPeers"`
				AddressPools   []legacyAddressPool `json:"addressPools"`
				BGPCommunities map[string]string   `json:"bgpCommunities"`
				Speaker        legacySpeaker       `json:"speaker"`
			} `json:"settings"`
		} `json:"spec"`
	}
	err := sdk.FromUnstructured(obj, &mc)
	if err != nil {
		return nil, err
	}
	return legacyBGPConfig{
		Version:        mc.Spec.Version,
		BGPPeers:       mc.Spec.Settings.BGPPeers,
		AddressPools:   mc.Spec.Settings.AddressPools,
		BGPCommunities: mc.Spec.Settings.BGPCommunities,
		Speaker:        mc.Spec.Settings.Speaker,
	}, nil
}

func migrateBGP(_ context.Context, input *go_hook.HookInput) error {
	snapsMC := input.Snapshots.Get("module_config")
	if len(snapsMC) != 1 || snapsMC[0] == nil {
		return nil
	}

	mc := new(legacyBGPConfig)
	if err := snapsMC[0].UnmarshalTo(mc); err != nil {
		return err
	}

	if mc.Version >= 3 {
		input.Logger.Info("migration skipped", "ModuleConfig version", mc.Version)
		return nil
	}

	if len(mc.AddressPools) == 0 && len(mc.BGPPeers) == 0 {
		return nil
	}

	var hasBGP bool
	for _, p := range mc.AddressPools {
		if p.Protocol == "bgp" {
			hasBGP = true
			break
		}
	}
	if !hasBGP && len(mc.BGPPeers) == 0 {
		return nil
	}

	existingPools := make(map[string]struct{})
	for _, p := range input.Snapshots.Get("existing_pools") {
		var name string
		if err := p.UnmarshalTo(&name); err == nil {
			existingPools[name] = struct{}{}
		}
	}

	existingPeers := make(map[string]struct{})
	for _, p := range input.Snapshots.Get("existing_peers") {
		var name string
		if err := p.UnmarshalTo(&name); err == nil {
			existingPeers[name] = struct{}{}
		}
	}

	existingConfigs := make(map[string]struct{})
	for _, p := range input.Snapshots.Get("existing_configs") {
		var name string
		if err := p.UnmarshalTo(&name); err == nil {
			existingConfigs[name] = struct{}{}
		}
	}

	// 1. Migrate pools
	for i, p := range mc.AddressPools {
		if p.Protocol != "bgp" {
			continue
		}

		name := p.Name
		if name == "" {
			name = fmt.Sprintf("pool-%d", i)
		}

		if _, exists := existingPools[name]; exists {
			continue
		}

		pool := MetalLoadBalancerPool{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "network.deckhouse.io/v1alpha1",
				Kind:       "MetalLoadBalancerPool",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"heritage":          "deckhouse",
					"auto-generated-by": "d8-migration-hook",
				},
			},
			Spec: MetalLoadBalancerPoolSpec{
				Addresses: p.Addresses,
			},
		}

		u, err := sdk.ToUnstructured(&pool)
		if err != nil {
			return fmt.Errorf("failed to convert pool %q to unstructured: %w", name, err)
		}
		input.PatchCollector.CreateOrUpdate(u)
	}

	// 2. Migrate peers
	var peerNames []string
	for i, p := range mc.BGPPeers {
		name := fmt.Sprintf("peer-%d", i)
		peerNames = append(peerNames, name)

		if _, exists := existingPeers[name]; exists {
			continue
		}

		spec := MetalLoadBalancerBGPPeerSpec{
			PeerAddress: p.PeerAddress,
			PeerASN:     int(p.PeerASN),
			MyASN:       int(p.MyASN),
			RouterID:    p.RouterID,
			PeerPort:    p.PeerPort,
			HoldTime:    p.HoldTime,
		}

		peer := MetalLoadBalancerBGPPeer{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "network.deckhouse.io/v1alpha1",
				Kind:       "MetalLoadBalancerBGPPeer",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"heritage":          "deckhouse",
					"auto-generated-by": "d8-migration-hook",
				},
			},
			Spec: spec,
		}

		u, err := sdk.ToUnstructured(&peer)
		if err != nil {
			return fmt.Errorf("failed to convert peer %q to unstructured: %w", name, err)
		}
		input.PatchCollector.CreateOrUpdate(u)
	}

	// 3. Migrate configuration
	var advs []Advertisement
	for i, p := range mc.AddressPools {
		if p.Protocol != "bgp" {
			continue
		}
		name := p.Name
		if name == "" {
			name = fmt.Sprintf("pool-%d", i)
		}

		if len(p.BGPAdvertisements) == 0 {
			advs = append(advs, Advertisement{
				PoolNames: []string{name},
			})
			continue
		}

		for _, ba := range p.BGPAdvertisements {
			var communities []string
			for _, c := range ba.Communities {
				var comm string
				if mapped, exists := mc.BGPCommunities[c]; exists {
					comm = mapped
				} else {
					comm = c
				}

				// Validate against CRD regex pattern
				if communityPattern.MatchString(comm) {
					communities = append(communities, comm)
				} else {
					input.Logger.Warn("Skipping legacy community because it does not match the valid CRD pattern: " + comm)
				}
			}

			adv := Advertisement{
				PoolNames: []string{name},
				BGP: &BGPAdvertisementConfig{
					Communities:       communities,
					LocalPref:         ba.LocalPref,
					AggregationLength: ba.AggregationLength,
				},
			}

			advs = append(advs, adv)
		}
	}

	if _, exists := existingConfigs["migrated-bgp"]; !exists && (len(peerNames) > 0 || len(advs) > 0) {
		cfg := MetalLoadBalancerConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "network.deckhouse.io/v1alpha1",
				Kind:       "MetalLoadBalancerConfiguration",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "migrated-bgp",
				Labels: map[string]string{
					"heritage":          "deckhouse",
					"auto-generated-by": "d8-migration-hook",
				},
			},
			Spec: MetalLoadBalancerConfigurationSpec{
				Mode:         "BGP",
				NodeSelector: mc.Speaker.NodeSelector,
				BGP: BGPConfig{
					PeerNames: peerNames,
				},
				Advertisements: advs,
			},
		}

		u, err := sdk.ToUnstructured(&cfg)
		if err != nil {
			return fmt.Errorf("failed to convert configuration %q to unstructured: %w", "migrated-bgp", err)
		}
		input.PatchCollector.CreateOrUpdate(u)
	}

	return nil
}
