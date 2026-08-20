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
	ComponentName    = "fencing-controller"
	LeaderElectionID = "fencing-controller.deckhouse.io"
	ControllerName   = "fencingfailednodestate"
)

const (
	NodeGroupLabel      = "node.deckhouse.io/group"
	NodeTypeLabel       = "node.deckhouse.io/type"
	FencingEnabledLabel = "node-manager.deckhouse.io/fencing-enabled"
	FencingModeLabel    = "node-manager.deckhouse.io/fencing-mode"
)

const (
	FencingDisableAnnotation     = "node-manager.deckhouse.io/fencing-disable"
	DisruptionApprovedAnnotation = "update.node.deckhouse.io/disruption-approved"
	UpdateApprovedAnnotation     = "update.node.deckhouse.io/approved"
)

const (
	FencingModeNotify   = "Notify"
	FencingModeWatchdog = "Watchdog"
)

const (
	NodeTypeStatic      = "Static"
	NodeTypeCloudStatic = "CloudStatic"
)

const (
	EnvPodNamespace = "POD_NAMESPACE"
	EnvPodName      = "POD_NAME"
	EnvLogLevel     = "LOG_LEVEL"
)
