/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type MetalLoadBalancerPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              MetalLoadBalancerPoolSpec `json:"spec"`
}

type MetalLoadBalancerPoolSpec struct {
	Addresses []string `json:"addresses"`
}

type MetalLoadBalancerBGPPeer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              MetalLoadBalancerBGPPeerSpec `json:"spec"`
}

type MetalLoadBalancerBGPPeerSpec struct {
	PeerAddress       string                   `json:"peerAddress"`
	PeerPort          *int                     `json:"peerPort,omitempty"`
	PeerASN           int                      `json:"peerASN"`
	MyASN             int                      `json:"myASN"`
	RouterID          string                   `json:"routerID,omitempty"`
	HoldTime          string                   `json:"holdTime,omitempty"`
	PasswordSecretRef *SecretRef               `json:"passwordSecretRef,omitempty"`
	SourceAddresses   []SourceAddress          `json:"sourceAddresses,omitempty"`
	BFD               *BFDProfileConfiguration `json:"bfd,omitempty"`
}

type SecretRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type SourceAddress struct {
	NodeName string `json:"nodeName"`
	Address  string `json:"address"`
}

type BFDProfileConfiguration struct {
	ReceiveInterval  *int  `json:"receiveInterval,omitempty"`
	TransmitInterval *int  `json:"transmitInterval,omitempty"`
	DetectMultiplier *int  `json:"detectMultiplier,omitempty"`
	EchoInterval     *int  `json:"echoInterval,omitempty"`
	EchoMode         *bool `json:"echoMode,omitempty"`
	PassiveMode      *bool `json:"passiveMode,omitempty"`
	MinimumTTL       *int  `json:"minimumTtl,omitempty"`
}

type MetalLoadBalancerConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              MetalLoadBalancerConfigurationSpec `json:"spec"`
}

type MetalLoadBalancerConfigurationSpec struct {
	NodeSelector   map[string]string `json:"nodeSelector,omitempty"`
	Mode           string            `json:"mode"`
	BGP            BGPConfig         `json:"bgp"`
	Advertisements []Advertisement   `json:"advertisements,omitempty"`
}

type BGPConfig struct {
	PeerNames []string `json:"peerNames,omitempty"`
}

type Advertisement struct {
	PoolNames []string               `json:"poolNames"`
	BGP       BGPAdvertisementConfig `json:"bgp"`
}

type BGPAdvertisementConfig struct {
	Communities       []string `json:"communities,omitempty"`
	LocalPref         *int     `json:"localPref,omitempty"`
	AggregationLength *int     `json:"aggregationLength,omitempty"`
}

type BGPPeerValue struct {
	Name           string                 `json:"name"`
	MyASN          int                    `json:"myASN"`
	PeerASN        int                    `json:"peerASN"`
	PeerAddress    string                 `json:"peerAddress"`
	RouterID       string                 `json:"routerID,omitempty"`
	PeerPort       *int                   `json:"peerPort,omitempty"`
	SourceAddress  string                 `json:"sourceAddress,omitempty"`
	HoldTime       string                 `json:"holdTime,omitempty"`
	PasswordSecret string                 `json:"passwordSecret,omitempty"`
	NodeSelectors  []metav1.LabelSelector `json:"nodeSelectors,omitempty"`
	BFDProfile     string                 `json:"bfdProfile,omitempty"`
}

type BGPAdvertisementValue struct {
	Name              string                 `json:"name"`
	IPAddressPools    []string               `json:"ipAddressPools"`
	NodeSelectors     []metav1.LabelSelector `json:"nodeSelectors,omitempty"`
	Peers             []string               `json:"peers,omitempty"`
	AggregationLength *int                   `json:"aggregationLength,omitempty"`
	LocalPref         *int                   `json:"localPref,omitempty"`
	Communities       []string               `json:"communities,omitempty"`
}

type BFDProfileValue struct {
	Name             string `json:"name"`
	ReceiveInterval  *int   `json:"receiveInterval,omitempty"`
	TransmitInterval *int   `json:"transmitInterval,omitempty"`
	DetectMultiplier *int   `json:"detectMultiplier,omitempty"`
	EchoInterval     *int   `json:"echoInterval,omitempty"`
	EchoMode         *bool  `json:"echoMode,omitempty"`
	PassiveMode      *bool  `json:"passiveMode,omitempty"`
	MinimumTTL       *int   `json:"minimumTtl,omitempty"`
}

type IPAddressPoolValue struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
}

type SecretToCopy struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Data      map[string]string `json:"data"`
}
