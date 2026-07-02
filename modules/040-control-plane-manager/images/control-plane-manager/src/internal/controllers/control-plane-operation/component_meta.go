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
	"sort"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki/signature"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/checksum"
)

// componentDependencies holds the dependencies for control plane component as runtime configuration files.
type componentDependencies struct {
	CertTree            map[pki.RootCertBaseName][]pki.LeafCertBaseName
	CAFiles             []string
	SignatureFiles      []string
	KubeconfigFiles     []kubeconfig.File
	ExtraFileKeys       []string
	NeedsRootKubeconfig bool
}

var componentDepsRegistry = map[controlplanev1alpha1.OperationComponent]componentDependencies{
	controlplanev1alpha1.OperationComponentEtcd: {
		CertTree: map[pki.RootCertBaseName][]pki.LeafCertBaseName{
			pki.EtcdCACertBaseName: {
				pki.EtcdServerCertBaseName,
				pki.EtcdPeerCertBaseName,
				pki.EtcdHealthcheckClientCertBaseName,
				pki.ApiserverEtcdClientCertBaseName,
			},
		},
		// TODO: remove this when we have a way to get the CA files from CertTree (pki.RootCertName)
		CAFiles: []string{
			string(pki.EtcdCACertBaseName) + ".crt",
			string(pki.EtcdCACertBaseName) + ".key",
		},
		ExtraFileKeys: checksum.ExtraFileKeysForPodComponent(controlplanev1alpha1.OperationComponentEtcd.PodComponentName()),
	},
	controlplanev1alpha1.OperationComponentKubeAPIServer: {
		CertTree: map[pki.RootCertBaseName][]pki.LeafCertBaseName{
			pki.CACertBaseName: {
				pki.ApiserverCertBaseName,
				pki.ApiserverKubeletClientCertBaseName,
			},
			pki.FrontProxyCACertBaseName: {
				pki.FrontProxyClientCertBaseName,
			},
		},
		CAFiles: []string{
			string(pki.CACertBaseName) + ".crt",
			string(pki.CACertBaseName) + ".key",
			string(pki.FrontProxyCACertBaseName) + ".crt",
			string(pki.FrontProxyCACertBaseName) + ".key",
			pki.SAPublicKeyFileName,
			pki.SAPrivateKeyFileName,
		},
		SignatureFiles: []string{
			signature.SignaturePrivateJWK,
			signature.SignaturePublicJWKS,
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
}

func componentDeps(component controlplanev1alpha1.OperationComponent) componentDependencies {
	return componentDepsRegistry[component]
}

func (d componentDependencies) leafCertFiles() []pki.LeafCertBaseName {
	if len(d.CertTree) == 0 {
		return nil
	}

	roots := make([]string, 0, len(d.CertTree))
	for root := range d.CertTree {
		roots = append(roots, string(root))
	}
	sort.Strings(roots)

	out := make([]pki.LeafCertBaseName, 0)
	for _, root := range roots {
		out = append(out, d.CertTree[pki.RootCertBaseName(root)]...)
	}
	return out
}
