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

import (
	"strings"
	"time"
)

const (
	KubeSystemNamespace                 = "kube-system"
	CpcControllerName                   = "control-plane-configuration-controller"
	CpnControllerName                   = "control-plane-node-controller"
	CpoControllerName                   = "control-plane-operation-controller"
	ControlPlaneManagerConfigSecretName = "d8-control-plane-manager-config"
	PkiSecretName                       = "d8-pki"
	ControlPlaneNodeLabelKey            = "node-role.kubernetes.io/control-plane"
	EtcdArbiterNodeLabelKey             = "node.deckhouse.io/etcd-arbiter"
	ControlPlaneNodeNameLabelKey        = "control-plane.deckhouse.io/node"
	ControlPlaneComponentLabelKey       = "control-plane.deckhouse.io/component"
	ConfigChecksumAnnotationKey         = "control-plane-manager.deckhouse.io/config-checksum"
	PKIChecksumAnnotationKey            = "control-plane-manager.deckhouse.io/pki-checksum"
	CAChecksumAnnotationKey             = "control-plane-manager.deckhouse.io/ca-checksum"
	NodeNameEnvVar                      = "NODE_NAME"
	KubernetesConfigPath                = "/etc/kubernetes"
	ManifestsPath                       = KubernetesConfigPath + "/manifests"
	DeckhousePath                       = KubernetesConfigPath + "/deckhouse"
	KubernetesPkiPath                   = KubernetesConfigPath + "/pki"
	PatchesPath                         = DeckhousePath + "/patches"
	ControlPlaneManifestsPath           = DeckhousePath + "/control-plane"
	ExtraFilesPath                      = DeckhousePath + "/extra-files"
	ConfigPath                          = "/config" // Mounted secret for d8-control-plane-manager-config
	PkiPath                             = "/pki"    // Mounted secret for d8-pki

	// ControlPlaneNode Conditions
	ConditionEtcdReady              = "EtcdReady"
	ConditionAPIServerReady         = "APIServerReady"
	ConditionControllerManagerReady = "ControllerManagerReady"
	ConditionSchedulerReady         = "SchedulerReady"
	ConditionCASynced               = "CASynced"
	ConditionHotReloadSynced        = "HotReloadSynced"

	ReasonSynced               = "Synced"
	ReasonOutOfSync            = "OutOfSync"
	ReasonUpdating             = "Updating"
	ReasonPendingUpdate        = "PendingUpdate"
	ReasonUpdateFailed         = "UpdateFailed"
	ReasonWaitingForComponents = "WaitingForComponents"
	ReasonUnknown              = "Unknown"

	// ControlPlaneOperation conditions
	ConditionApproved = "Approved"
	ConditionReady    = "Ready"
	ConditionFailed   = "Failed"

	// Pipeline command condition types (observability)
	ConditionCommandBackup           = "Backup"
	ConditionCommandSyncCA           = "SyncCA"
	ConditionCommandRenewPKICerts    = "RenewPKICerts"
	ConditionCommandRenewKubeconfigs = "RenewKubeconfigs"
	ConditionCommandSyncManifests    = "SyncManifests"
	ConditionCommandJoinEtcdCluster  = "JoinEtcdCluster"
	ConditionCommandWaitPodReady     = "WaitPodReady"
	ConditionCommandSyncHotReload    = "SyncHotReload"
	ConditionCommandCertObserve      = "CertObserve"

	// Approved condition reasons
	ReasonApproved             = "Approved"
	ReasonWaitingForSlot       = "WaitingForSlot"
	ReasonWaitingForDependency = "WaitingForDependency"
	ReasonWaitingForFIFO       = "WaitingForFIFO"

	// Command condition reasons
	ReasonCommandCompleted  = "Completed"
	ReasonCommandInProgress = "InProgress"
	ReasonCommandFailed     = "Failed"

	// Ready condition reasons
	ReasonOperationSucceeded  = "OperationSucceeded"
	ReasonWaitingForApproval  = "WaitingForApproval"
	ReasonCreatingBackup      = "CreatingBackup"
	ReasonSyncingCA           = "SyncingCA"
	ReasonRenewingPKI         = "RenewingPKI"
	ReasonRenewingKubeconfigs = "RenewingKubeconfigs"
	ReasonSyncingManifests    = "SyncingManifests"
	ReasonJoiningEtcd         = "JoiningEtcd"
	ReasonWaitingForPod       = "WaitingForPod"
	ReasonSyncingHotReload    = "SyncingHotReload"
	ReasonCertObserving       = "CertObserving"
	ReasonCancelled           = "Cancelled"

	// Failed condition reasons
	ReasonNoFailure         = "NoFailure"
	ReasonHealthCheckFailed = "HealthCheckFailed"
	ReasonTimeout           = "Timeout"

	// Built-in k8s label on static pods with the component name
	StaticPodComponentLabelKey = "component"

	// Env vars for path overrides for debug/dev
	ManifestDirEnvVar   = "MANIFEST_DIR"
	ExtraFilesDirEnvVar = "EXTRA_FILES_DIR"
	PkiDirEnvVar        = "PKI_DIR"
	KubeconfigDirEnvVar = "KUBECONFIG_DIR"

	// Secret keys for PKI config in d8-control-plane-manager-config
	SecretKeyCertSANs            = "cert-sans"
	SecretKeyEncryptionAlgorithm = "encryption-algorithm"

	// Not configurable endpoint for local control plane
	LocalControlPlaneEndpoint = "127.0.0.1:6445"

	// Backup config
	BackupBasePath         = DeckhousePath + "/backups"
	MaxBackupsPerComponent = 7

	// Diff config
	DiffBasePath         = DeckhousePath + "/diffs"
	MaxDiffsPerComponent = 7

	// CertObserverInterval - minimum duration between periodic CertObserver(all) operations.
	CertObserverInterval = 7 * 24 * time.Hour

	// Cert renewal
	CertRenewalIDAnnotationKey = "control-plane.deckhouse.io/cert-renewal-id"
	CertRenewalThreshold       = 30 * 24 * time.Hour

	// CertsRenewal - ControlPlaneNode condition
	ConditionCertsRenewal = "CertsRenewal"
	ReasonHealthy         = "Healthy"
	ReasonCertExpiring    = "CertExpiring"
	ReasonRenewing        = "Renewing"
	ReasonRenewed         = "Renewed"
	ReasonRenewalFailed   = "RenewalFailed"
)

// ToRelativePath returns path without leading slash for using in tmp directory sync
func ToRelativePath(absolutePath string) string {
	return strings.TrimPrefix(absolutePath, "/")
}
