/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import v1 "k8s.io/api/core/v1"

type ClusterDebugInfo struct {
	ID         string `json:"id"`
	SecretName string `json:"secretName"`
	SyncStatus string `json:"syncStatus"`
}

type IstioPodInfo struct {
	Name   string
	IP     string
	Status v1.PodPhase
}
