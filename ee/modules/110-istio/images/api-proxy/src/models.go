/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

type PublicMetadata struct {
	ClusterUUID string `json:"clusterUUID,omitempty"`
	AuthnKeyPub string `json:"authnKeyPub,omitempty"`
	RootCA      string `json:"rootCA,omitempty"`
}

// RemotePublicMetadata map[custerUUID]pubilcMetadata
type RemotePublicMetadata map[string]*PublicMetadata

type JwtPayload struct {
	Iss   string `json:"iss"`
	Sub   string `json:"sub"`
	Aud   string `json:"aud"`
	Scope string `json:"scope"`
	Nbf   int64  `json:"nbf"`
	Exp   int64  `json:"exp"`
}
