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

package constants

import "strings"

const (
	KubeSystemNamespace                 = "kube-system"
	CpcControllerName                   = "control-plane-configuration-controller"
	CpnControllerName                   = "control-plane-node-controller"
	CpoControllerName                   = "control-plane-operation-controller"
	ControlPlaneManagerConfigSecretName = "d8-control-plane-manager-config"
	PkiSecretName                       = "d8-pki"
	ControlPlaneNodeLabelKey            = "node-role.kubernetes.io/control-plane"
	ControlPlaneNodeNameLabelKey        = "control-plane.deckhouse.io/node"
	ControlPlaneComponentLabelKey       = "control-plane.deckhouse.io/component"
	NodeNameEnvVar                      = "NODE_NAME"
	KubernetesConfigPath                = "/etc/kubernetes"
	ManifestsPath                       = KubernetesConfigPath + "/manifests"
	DeckhousePath                       = KubernetesConfigPath + "/deckhouse"
	KubernetesPkiPath                   = KubernetesConfigPath + "/pki"
	PatchesPath                         = DeckhousePath + "/patches"
	ExtraFilesPath                      = DeckhousePath + "/extra-files"
	ConfigPath                          = "/config" // Mounted secret for d8-control-plane-manager-config
	PkiPath                             = "/pki"    // Mounted secret for d8-pki

	// ControlPlaneNode Conditions
	ConditionEtcdReady              = "EtcdReady"
	ConditionAPIServerReady         = "APIServerReady"
	ConditionControllerManagerReady = "ControllerManagerReady"
	ConditionSchedulerReady         = "SchedulerReady"
	ConditionPKISynced              = "PKISynced"
	ConditionsHotReloadSynced       = "HotReloadSynced"

	ReasonSynced        = "Synced"
	ReasonOutOfSync     = "OutOfSync"
	ReasonUpdating      = "Updating"
	ReasonPendingUpdate = "PendingUpdate"
	ReasonUpdateFailed  = "UpdateFailed"
	ReasonUnknown       = "Unknown"
)

// ToRelativePath returns path without leading slash for using in tmp directory sync
func ToRelativePath(absolutePath string) string {
	return strings.TrimPrefix(absolutePath, "/")
}
