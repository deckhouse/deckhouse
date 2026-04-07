/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

const (
	// Node labels
	NodeGroupLabel = "node.deckhouse.io/group"
	NodeTypeLabel  = "node.deckhouse.io/type"

	// Update annotations
	ApprovedAnnotation           = "update.node.deckhouse.io/approved"
	WaitingForApprovalAnnotation = "update.node.deckhouse.io/waiting-for-approval"
	DisruptionRequiredAnnotation = "update.node.deckhouse.io/disruption-required"
	DisruptionApprovedAnnotation = "update.node.deckhouse.io/disruption-approved"
	RollingUpdateAnnotation      = "update.node.deckhouse.io/rolling-update"
	DrainingAnnotation           = "update.node.deckhouse.io/draining"
	DrainedAnnotation            = "update.node.deckhouse.io/drained"

	// Node metadata annotations
	ConfigurationChecksumAnnotation = "node.deckhouse.io/configuration-checksum"
)
