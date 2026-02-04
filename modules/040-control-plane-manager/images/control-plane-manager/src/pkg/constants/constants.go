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

const (
	ControllerName                      = "control-plane-manager"
	ControlPlaneManagerConfigSecretName = "d8-control-plane-manager-config"
	PkiSecretName                       = "d8-pki"
	ControlPlaneConfigurationName       = "control-plane"
	KubernetesConfigPath                = "/etc/kubernetes"
	ManifestsPath                       = KubernetesConfigPath + "/manifests"
	DeckhousePath                       = KubernetesConfigPath + "/deckhouse"
	ConfigPath                          = "/config"
	PkiPath                             = "/pki"
	KubernetesPkiPath                   = KubernetesConfigPath + "/pki"
	KubeadmPath                         = "/kubeadm"
	TmpPath                             = "/tmp/control-plane-manager-manifests"
	KubeSystemNamespace                 = "kube-system"

	RelativeKubernetesDir  = "etc/kubernetes"
	RelativePkiDir         = RelativeKubernetesDir + "/pki"
	RelativeDeckhouseDir   = RelativeKubernetesDir + "/deckhouse"
	RelativeKubeadmDir     = RelativeDeckhouseDir + "/kubeadm"
	RelativePatchesDir     = RelativeKubeadmDir + "/patches"
	RelativeExtraFilesDir  = RelativeDeckhouseDir + "/extra-files"
)
