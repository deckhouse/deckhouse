/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

type IngressGateway struct {
	Address string `json:"address,omitempty"`
	Port    int32  `json:"port,omitempty"`
}

type Node struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	IsActive bool   `json:"isActive"`
}

// PublicService struct for federation
type PublicService struct {
	Hostname string `json:"hostname"`
	Ports    []Port `json:"ports"`
}

type Port struct {
	Name string `json:"name"`
	Port int32  `json:"port"`
}

// SpiffeKey describe key for Spiffe bundle
type SpiffeKey struct {
	Kty string   `json:"kty"`
	Use string   `json:"use"`
	E   string   `json:"e"`
	N   string   `json:"n"`
	X5c [][]byte `json:"x5c"`
}

// SpiffeEndpoint describe Spiffe bundle
type SpiffeEndpoint struct {
	SpiffeSequence    int         `json:"spiffe_sequence"`
	SpiffeRefreshHint int         `json:"spiffe_refresh_hint"`
	Keys              []SpiffeKey `json:"keys"`
}

// TODO import from hooks package
// Warning! These two structs below are duplicated in hooks/ee/lib/crd
type AlliancePublicMetadata struct {
	ClusterUUID string                     `json:"clusterUUID"`
	AuthnKeyPub string                     `json:"authnKeyPub"`
	RootCA      string                     `json:"rootCA"`
	AllianceRef *PublicMetadataAllianceRef `json:"allianceRef,omitempty"`
}

type PublicMetadataAllianceRef struct {
	Kind string `json:"kind,omitempty"`
	Name string `json:"name,omitempty"`
}

type FederationPrivateMetadata struct {
	IngressGateways *[]IngressGateway `json:"ingressGateways,omitempty"`
	PublicServices  *[]PublicService  `json:"publicServices,omitempty"`
}

type MulticlusterPrivateMetadata struct {
	IngressGateways *[]IngressGateway `json:"ingressGateways,omitempty"`
	APIHost         string            `json:"apiHost,omitempty"`
	NetworkName     string            `json:"networkName,omitempty"`
}

// map[custerUUID]publicMetadata
type RemotePublicMetadata map[string]AlliancePublicMetadata

type JwtPayload struct {
	Iss   string `json:"iss"`
	Sub   string `json:"sub"`
	Aud   string `json:"aud"`
	Scope string `json:"scope"`
	Nbf   int64  `json:"nbf"`
	Exp   int64  `json:"exp"`
}
