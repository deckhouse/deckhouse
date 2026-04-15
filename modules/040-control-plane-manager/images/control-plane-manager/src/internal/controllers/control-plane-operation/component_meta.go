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

package controlplaneoperation

import (
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/checksum"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki"
)

type componentDeps struct {
	CertTree            map[pki.RootCertName][]pki.LeafCertName
	LeafCertFiles       []pki.LeafCertName
	CAFiles             []string
	KubeconfigFiles     []kubeconfig.File
	ExtraFileKeys       []string
	NeedsRootKubeconfig bool
}

var componentDepsRegistry = map[controlplanev1alpha1.OperationComponent]componentDeps{
	controlplanev1alpha1.OperationComponentEtcd: {
		CertTree: map[pki.RootCertName][]pki.LeafCertName{
			pki.EtcdCACertName: {
				pki.EtcdServerCertName,
				pki.EtcdPeerCertName,
				pki.EtcdHealthcheckClientCertName,
				pki.ApiserverEtcdClientCertName,
			},
		},
		LeafCertFiles: []pki.LeafCertName{
			pki.EtcdServerCertName,
			pki.EtcdPeerCertName,
			pki.EtcdHealthcheckClientCertName,
			pki.ApiserverEtcdClientCertName,
		},
		CAFiles: []string{
			string(pki.EtcdCACertName) + ".crt",
			string(pki.EtcdCACertName) + ".key",
		},
		ExtraFileKeys: checksum.ExtraFileKeysForPodComponent(controlplanev1alpha1.OperationComponentEtcd.PodComponentName()),
	},
	controlplanev1alpha1.OperationComponentKubeAPIServer: {
		CertTree: map[pki.RootCertName][]pki.LeafCertName{
			pki.CACertName: {
				pki.ApiserverCertName,
				pki.ApiserverKubeletClientCertName,
			},
			pki.FrontProxyCACertName: {
				pki.FrontProxyClientCertName,
			},
		},
		LeafCertFiles: []pki.LeafCertName{
			pki.ApiserverCertName,
			pki.ApiserverKubeletClientCertName,
			pki.FrontProxyClientCertName,
		},
		CAFiles: []string{
			string(pki.CACertName) + ".crt",
			string(pki.CACertName) + ".key",
			string(pki.FrontProxyCACertName) + ".crt",
			string(pki.FrontProxyCACertName) + ".key",
			"sa.pub",
			"sa.key",
		},
		ExtraFileKeys:       checksum.ExtraFileKeysForPodComponent(controlplanev1alpha1.OperationComponentKubeAPIServer.PodComponentName()),
		KubeconfigFiles:     []kubeconfig.File{kubeconfig.Admin, kubeconfig.SuperAdmin},
		NeedsRootKubeconfig: true,
	},
	controlplanev1alpha1.OperationComponentKubeControllerManager: {
		KubeconfigFiles: []kubeconfig.File{kubeconfig.ControllerManager},
		ExtraFileKeys:   checksum.ExtraFileKeysForPodComponent(controlplanev1alpha1.OperationComponentKubeControllerManager.PodComponentName()),
	},
	controlplanev1alpha1.OperationComponentKubeScheduler: {
		KubeconfigFiles: []kubeconfig.File{kubeconfig.Scheduler},
		ExtraFileKeys:   checksum.ExtraFileKeysForPodComponent(controlplanev1alpha1.OperationComponentKubeScheduler.PodComponentName()),
	},
	controlplanev1alpha1.OperationComponentHotReload: {
		ExtraFileKeys: checksum.HotReloadChecksumDependsOn,
	},
}

func componentDepsForComponent(component controlplanev1alpha1.OperationComponent) componentDeps {
	return componentDepsRegistry[component]
}
