/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package k8s_legacy

import (
	"time"
)

type MasterNode struct {
	Name                    string
	Address                 string
	CreationTimestamp       time.Time
	AuthCertificate         Certificate
	DistributionCertificate Certificate
}

const (
	RegistryNamespace      = "d8-system"
	RegistryMcName         = "system-registry"
	ModuleConfigApiVersion = "deckhouse.io/v1alpha1"
	ModuleConfigKind       = "ModuleConfig"
	RegistrySvcName        = labelModuleValue
)

func GetFirstCreatedNodeForSync(masterNodes map[string]MasterNode) *MasterNode {
	var earliestNode *MasterNode
	var earliestTime time.Time

	// Select the earliest created node
	for _, masterNode := range masterNodes {
		if earliestTime.IsZero() || masterNode.CreationTimestamp.Before(earliestTime) {
			earliestTime = masterNode.CreationTimestamp
			earliestNode = &masterNode
		}
	}

	return earliestNode
}
