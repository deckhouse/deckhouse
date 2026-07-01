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

type ControlPlaneType string

const (
	ControlPlaneTypeNormal  ControlPlaneType = "normal"
	ControlPlaneTypeVirtual ControlPlaneType = "virtual"
)

// Normal control plane manager constants
const (
	ControlPlaneManagerName = "control-plane-manager"
	CpcControllerName       = "control-plane-configuration-controller"
	CpnControllerName       = "control-plane-node-controller"
	CpoControllerName       = "control-plane-operation-controller"
)

// Virtual control plane manager constants
const (
	VirtualControlPlaneManagerName         = "virtual-control-plane-manager"
	VirtualConfigurationController         = "virtual-control-plane-configuration-controller"
	VirtualControlPlaneNodeController      = "virtual-control-plane-node-controller"
	VirtualControlPlaneNamespacePrefix     = "vcp-"
	VirtualControlPlaneConfigSecretName    = "d8-virtual-control-plane-config"
	VirtualControlPlaneConfigSecretSuffix  = "-config"
	VirtualControlPlaneNodeOrdinalLabelKey = "control-plane.deckhouse.io/virtual-control-plane-node-ordinal"
	DefaultTenantClusterDomain             = "cluster.virtual"
	DefaultTenantServiceSubnetCIDR         = "10.96.0.0/12"
)

const (
	KubeSystemNamespace                 = "kube-system"
	ControlPlaneManagerConfigSecretName = "d8-control-plane-manager-config"
	PkiSecretName                       = "d8-pki"
	ControlPlaneNodeLabelKey            = "node-role.kubernetes.io/control-plane"
	EtcdArbiterNodeLabelKey             = "node.deckhouse.io/etcd-arbiter"
	ControlPlaneNodeNameLabelKey        = "control-plane.deckhouse.io/node"
	ControlPlaneTypeLabelKey            = "control-plane.deckhouse.io/type"
	ControlPlaneComponentLabelKey       = "control-plane.deckhouse.io/component"
	HeritageLabelKey                    = "heritage"
	HeritageLabelValue                  = "deckhouse"
	MaintenanceModeLabelKey             = "control-plane-manager.deckhouse.io/maintenance"
	ConfigChecksumAnnotationKey         = "control-plane-manager.deckhouse.io/config-checksum"
	PKIChecksumAnnotationKey            = "control-plane-manager.deckhouse.io/pki-checksum" // actually this is cert params checksum (certSANs, encryption-algorithm)
	CertsChecksumAnnotationKey          = "control-plane-manager.deckhouse.io/certs-checksum"
	CAChecksumAnnotationKey             = "control-plane-manager.deckhouse.io/ca-checksum"
	OperationStartedAtAnnotationKey     = "control-plane-manager.deckhouse.io/operation-started-at"
	NodeNameEnvVar                      = "NODE_NAME"
	DaemonSetNameEnvVar                 = "DAEMONSET_NAME"
	EtcdArbiterEnvVar                   = "ETCD_ARBITER"
	KubernetesConfigPath                = "/etc/kubernetes"
	ManifestsPath                       = KubernetesConfigPath + "/manifests"
	DeckhousePath                       = KubernetesConfigPath + "/deckhouse"
	KubernetesPkiPath                   = KubernetesConfigPath + "/pki"
	PatchesPath                         = DeckhousePath + "/patches"
	ControlPlaneManifestsPath           = DeckhousePath + "/control-plane"
	ExtraFilesPath                      = DeckhousePath + "/extra-files"
	ConfigPath                          = "/config" // Mounted secret for d8-control-plane-manager-config
	PkiPath                             = "/pki"    // Mounted secret for d8-pki

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
	BackupBasePath         = DeckhousePath + "/backup"
	MaxBackupsPerComponent = 7

	// Diff config
	DiffBasePath         = DeckhousePath + "/diffs"
	MaxDiffsPerComponent = 7

	// CertObserveInterval is the minimum duration between periodic CertObserve steps for a component.
	CertObserveInterval = 7 * 24 * time.Hour

	// Cert renewal
	CertRenewalIDAnnotationKey       = "control-plane.deckhouse.io/cert-renewal-id"
	KubeconfigRenewalIDAnnotationKey = "control-plane.deckhouse.io/kubeconfig-renewal-id"
	SignatureRenewalIDAnnotationKey  = "control-plane.deckhouse.io/signature-renewal-id"
	SignatureExpirationKey           = "signature"
	SignatureRenewalDays             = 60
	SignatureRenewalThreshold        = SignatureRenewalDays * 24 * time.Hour
	CertRenewalThreshold             = 30 * 24 * time.Hour
)

var SignatureBuildEnabled = "false"

// SignatureEnabled reports whether the signature feature is built into this binary (CSE only).
// Flag is overridden by an ldflag in the CSE build (see werf.inc.yaml).
func SignatureEnabled() bool {
	return strings.ToLower(SignatureBuildEnabled) == "true"
}

// ToRelativePath returns path without leading slash for using in tmp directory sync
func ToRelativePath(absolutePath string) string {
	return strings.TrimPrefix(absolutePath, "/")
}
