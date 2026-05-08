/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/metallb/discovery_bgp",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "pools",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "MetalLoadBalancerPool",
			FilterFunc: filterPool,
		},
		{
			Name:       "peers",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "MetalLoadBalancerBGPPeer",
			FilterFunc: filterPeer,
		},
		{
			Name:       "configs",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "MetalLoadBalancerConfiguration",
			FilterFunc: filterConfig,
		},
		{
			Name:       "secrets",
			ApiVersion: "v1",
			Kind:       "Secret",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"network.deckhouse.io/metallb-bgp-password": "true",
				},
			},
			FilterFunc: filterSecret,
		},
	},
}, handleBGP)

func filterPool(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pool MetalLoadBalancerPool
	if err := sdk.FromUnstructured(obj, &pool); err != nil {
		return nil, err
	}
	return pool, nil
}

func filterPeer(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var peer MetalLoadBalancerBGPPeer
	if err := sdk.FromUnstructured(obj, &peer); err != nil {
		return nil, err
	}
	return peer, nil
}

func filterConfig(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var config MetalLoadBalancerConfiguration
	if err := sdk.FromUnstructured(obj, &config); err != nil {
		return nil, err
	}
	return config, nil
}

func filterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1.Secret
	if err := sdk.FromUnstructured(obj, &secret); err != nil {
		return nil, err
	}
	return secret, nil
}

func handleBGP(_ context.Context, input *go_hook.HookInput) error {
	// Collect data from snapshots
	var pools []MetalLoadBalancerPool
	for _, p := range input.Snapshots.Get("pools") {
		var pool MetalLoadBalancerPool
		if err := p.UnmarshalTo(&pool); err != nil {
			input.Logger.Warn(fmt.Sprintf("failed to unmarshal MetalLoadBalancerPool snapshot: %v", err))
			continue
		}
		pools = append(pools, pool)
	}

	var peers []MetalLoadBalancerBGPPeer
	for _, p := range input.Snapshots.Get("peers") {
		var peer MetalLoadBalancerBGPPeer
		if err := p.UnmarshalTo(&peer); err != nil {
			input.Logger.Warn(fmt.Sprintf("failed to unmarshal MetalLoadBalancerBGPPeer snapshot: %v", err))
			continue
		}
		peers = append(peers, peer)
	}

	var configs []MetalLoadBalancerConfiguration
	for _, c := range input.Snapshots.Get("configs") {
		var config MetalLoadBalancerConfiguration
		if err := c.UnmarshalTo(&config); err != nil {
			input.Logger.Warn(fmt.Sprintf("failed to unmarshal MetalLoadBalancerConfiguration snapshot: %v", err))
			continue
		}
		configs = append(configs, config)
	}

	secrets := make(map[string]map[string]string)
	for _, s := range input.Snapshots.Get("secrets") {
		var secret v1.Secret
		if err := s.UnmarshalTo(&secret); err != nil {
			input.Logger.Warn(fmt.Sprintf("failed to unmarshal Secret snapshot: %v", err))
			continue
		}
		data := make(map[string]string)
		for k, v := range secret.Data {
			data[k] = string(v)
		}
		secrets[fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)] = data
	}

	// Map peers by name for quick lookup
	peersByName := make(map[string]MetalLoadBalancerBGPPeer, len(peers))
	for _, p := range peers {
		peersByName[p.Name] = p
	}

	// Process address pools
	outPools := make([]IPAddressPoolValue, 0, len(pools))
	for _, pool := range pools {
		outPools = append(outPools, IPAddressPoolValue{
			Name:          pool.Name,
			Addresses:     pool.Spec.Addresses,
			AutoAssign:    pool.Spec.AutoAssign,
			AvoidBuggyIPs: pool.Spec.AvoidBuggyIPs,
		})
	}

	outPeers := make([]BGPPeerValue, 0)
	outAdvs := make([]BGPAdvertisementValue, 0)
	speakerNodeSelectorTerms := make([]v1.NodeSelectorTerm, 0)
	secretsByName := make(map[string]SecretToCopy)
	bfdProfilesByName := make(map[string]BFDProfileValue)

	requiresGlobalDeployment := false

	// Main processing loop: advertisements, peers, BFD, and secrets
	for _, cfg := range configs {
		if cfg.Spec.Mode != "BGP" {
			continue
		}

		// Collect speaker node selector terms
		if len(cfg.Spec.NodeSelector) > 0 {
			matchExpressions := make([]v1.NodeSelectorRequirement, 0, len(cfg.Spec.NodeSelector))
			// Ensure deterministic order for matchExpressions
			keys := make([]string, 0, len(cfg.Spec.NodeSelector))
			for k := range cfg.Spec.NodeSelector {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				matchExpressions = append(matchExpressions, v1.NodeSelectorRequirement{
					Key:      k,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{cfg.Spec.NodeSelector[k]},
				})
			}
			speakerNodeSelectorTerms = append(speakerNodeSelectorTerms, v1.NodeSelectorTerm{
				MatchExpressions: matchExpressions,
			})
		} else {
			requiresGlobalDeployment = true
		}

		// Generate peers (must be done before advertisements to collect native peer names)
		var generatedPeerNames []string
		for _, peerName := range cfg.Spec.BGP.PeerNames {
			peer, ok := peersByName[peerName]
			if !ok {
				input.Logger.Warn(fmt.Sprintf("MetalLoadBalancerConfiguration %q references non-existent peer %q, skipping", cfg.Name, peerName))
				continue
			}

			// Extract secret if present
			var secretName string
			if peer.Spec.PasswordSecretRef != nil {
				s := *peer.Spec.PasswordSecretRef
				candidateName := fmt.Sprintf("bgp-pwd-%s-%s", s.Namespace, s.Name)

				secretData, found := secrets[fmt.Sprintf("%s/%s", s.Namespace, s.Name)]
				if found {
					secretName = candidateName
					secretsByName[secretName] = SecretToCopy{
						Name:      secretName,
						Namespace: s.Namespace, // original namespace, though not used in template
						Data:      secretData,
					}
				} else {
					input.Logger.Warn(fmt.Sprintf("secret %s/%s referenced by peer %q not found (missing label network.deckhouse.io/metallb-bgp-password?)", s.Namespace, s.Name, peer.Name))
				}
			}

			// Extract BFD if present
			var bfdName string
			// BFD is not supported yet because we don't use FRR.
			// We skip BFD profile generation and do not set bfdName to prevent it
			// from being included in the peer configuration.
			/*
				if peer.Spec.BFD != nil {
					bfdName = fmt.Sprintf("bfd-%s", peer.Name)
					bfdProfilesByName[bfdName] = BFDProfileValue{
						Name:             bfdName,
						ReceiveInterval:  peer.Spec.BFD.ReceiveInterval,
						TransmitInterval: peer.Spec.BFD.TransmitInterval,
						DetectMultiplier: peer.Spec.BFD.DetectMultiplier,
						EchoInterval:     peer.Spec.BFD.EchoInterval,
						EchoMode:         peer.Spec.BFD.EchoMode,
						PassiveMode:      peer.Spec.BFD.PassiveMode,
						MinimumTTL:       peer.Spec.BFD.MinimumTTL,
					}
				}
			*/

			explicitNodes := make([]string, 0, len(peer.Spec.SourceAddresses))
			for _, sa := range peer.Spec.SourceAddresses {
				explicitNodes = append(explicitNodes, sa.NodeName)

				peerNodeSelector := make(map[string]string, len(cfg.Spec.NodeSelector)+1)
				maps.Copy(peerNodeSelector, cfg.Spec.NodeSelector)
				peerNodeSelector["kubernetes.io/hostname"] = sa.NodeName

				perNodePeerName := fmt.Sprintf("%s-%s-node-%s", peer.Name, cfg.Name, sa.NodeName)
				generatedPeerNames = append(generatedPeerNames, perNodePeerName)
				outPeers = append(outPeers, BGPPeerValue{
					Name:           perNodePeerName,
					MyASN:          peer.Spec.MyASN,
					PeerASN:        peer.Spec.PeerASN,
					PeerAddress:    peer.Spec.PeerAddress,
					RouterID:       peer.Spec.RouterID,
					PeerPort:       peer.Spec.PeerPort,
					HoldTime:       peer.Spec.HoldTime,
					SourceAddress:  sa.Address,
					PasswordSecret: secretName,
					BFDProfile:     bfdName,
					NodeSelectors: []metav1.LabelSelector{
						{
							MatchLabels: peerNodeSelector,
						},
					},
				})

				speakerNodeSelectorTerms = append(speakerNodeSelectorTerms, v1.NodeSelectorTerm{
					MatchExpressions: []v1.NodeSelectorRequirement{
						{
							Key:      "kubernetes.io/hostname",
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{sa.NodeName},
						},
					},
				})
			}

			var fallbackNodeSelectors []metav1.LabelSelector
			if len(cfg.Spec.NodeSelector) > 0 || len(explicitNodes) > 0 {
				ls := metav1.LabelSelector{
					MatchLabels: cfg.Spec.NodeSelector,
				}
				if len(explicitNodes) > 0 {
					// Ensure deterministic order
					sort.Strings(explicitNodes)
					ls.MatchExpressions = []metav1.LabelSelectorRequirement{
						{
							Key:      "kubernetes.io/hostname",
							Operator: metav1.LabelSelectorOpNotIn,
							Values:   explicitNodes,
						},
					}
				}
				fallbackNodeSelectors = append(fallbackNodeSelectors, ls)
			}

			fallbackPeerName := fmt.Sprintf("%s-%s", peer.Name, cfg.Name)
			generatedPeerNames = append(generatedPeerNames, fallbackPeerName)
			outPeers = append(outPeers, BGPPeerValue{
				Name:           fallbackPeerName,
				MyASN:          peer.Spec.MyASN,
				PeerASN:        peer.Spec.PeerASN,
				PeerAddress:    peer.Spec.PeerAddress,
				RouterID:       peer.Spec.RouterID,
				PeerPort:       peer.Spec.PeerPort,
				HoldTime:       peer.Spec.HoldTime,
				PasswordSecret: secretName,
				BFDProfile:     bfdName,
				NodeSelectors:  fallbackNodeSelectors,
			})
		}

		// Generate advertisements (after peers, so we have the generated native peer names)
		for i, adv := range cfg.Spec.Advertisements {
			outAdv := BGPAdvertisementValue{
				Name:           fmt.Sprintf("%s-adv-%d", cfg.Name, i),
				IPAddressPools: adv.PoolNames,
				Peers:          generatedPeerNames,
			}
			if adv.BGP != nil {
				outAdv.Communities = adv.BGP.Communities
				outAdv.LocalPref = adv.BGP.LocalPref
				outAdv.AggregationLength = adv.BGP.AggregationLength
			}
			if len(cfg.Spec.NodeSelector) > 0 {
				outAdv.NodeSelectors = []metav1.LabelSelector{
					{MatchLabels: cfg.Spec.NodeSelector},
				}
			}
			outAdvs = append(outAdvs, outAdv)
		}
	}

	// Finalize secrets and BFD profiles
	outSecrets := make([]SecretToCopy, 0, len(secretsByName))
	for _, v := range secretsByName {
		outSecrets = append(outSecrets, v)
	}

	outBFDs := make([]BFDProfileValue, 0, len(bfdProfilesByName))
	for _, v := range bfdProfilesByName {
		outBFDs = append(outBFDs, v)
	}

	// Deduplicate and sort speakerNodeSelectorTerms for Helm stability
	speakerNodeSelectorTerms = deduplicateNodeSelectorTerms(speakerNodeSelectorTerms)

	// Sort all outputs to ensure Helm values stability
	sort.Slice(outPools, func(i, j int) bool { return outPools[i].Name < outPools[j].Name })
	sort.Slice(outPeers, func(i, j int) bool { return outPeers[i].Name < outPeers[j].Name })
	sort.Slice(outAdvs, func(i, j int) bool { return outAdvs[i].Name < outAdvs[j].Name })
	sort.Slice(outBFDs, func(i, j int) bool { return outBFDs[i].Name < outBFDs[j].Name })
	sort.Slice(outSecrets, func(i, j int) bool { return outSecrets[i].Name < outSecrets[j].Name })

	// Save final values to Helm internal variables
	input.Values.Set("metallb.internal.addressPools", outPools)
	input.Values.Set("metallb.internal.bgpPeers", outPeers)
	input.Values.Set("metallb.internal.bgpAdvertisements", outAdvs)
	input.Values.Set("metallb.internal.bfdProfiles", outBFDs)
	input.Values.Set("metallb.internal.secretsToCopy", outSecrets)

	switch {
	case requiresGlobalDeployment:
		input.Values.Set("metallb.internal.speakerNodeAffinity", map[string]any{})
	case len(speakerNodeSelectorTerms) > 0:
		input.Values.Set("metallb.internal.speakerNodeAffinity", map[string]any{
			"requiredDuringSchedulingIgnoredDuringExecution": map[string]any{
				"nodeSelectorTerms": speakerNodeSelectorTerms,
			},
		})
	default:
		// By default (if no specific node selectors are provided), deploy on all nodes.
		input.Values.Set("metallb.internal.speakerNodeAffinity", map[string]any{})
	}

	return nil
}

// deduplicateNodeSelectorTerms removes duplicate NodeSelectorTerms and sorts them
// for deterministic Helm output.
func deduplicateNodeSelectorTerms(terms []v1.NodeSelectorTerm) []v1.NodeSelectorTerm {
	seen := make(map[string]struct{}, len(terms))
	result := make([]v1.NodeSelectorTerm, 0, len(terms))

	for _, term := range terms {
		data, _ := json.Marshal(term)
		key := string(data)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, term)
	}

	sort.Slice(result, func(i, j int) bool {
		ki, _ := json.Marshal(result[i])
		kj, _ := json.Marshal(result[j])
		return string(ki) < string(kj)
	})

	return result
}
