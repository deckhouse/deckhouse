/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package crd

// Warning! These structs are duplicated in images/metadata-exporter/src/models.go

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
