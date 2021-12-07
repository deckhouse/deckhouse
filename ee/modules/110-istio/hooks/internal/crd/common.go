/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package crd

// Warning! This struct is duplicated in images/metadata-exporter
type AlliancePublicMetadata struct {
	ClusterUUID string `json:"clusterUUID"`
	AuthnKeyPub string `json:"authnKeyPub"`
	RootCA      string `json:"rootCA"`
}
