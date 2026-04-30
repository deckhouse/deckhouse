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

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/checksum"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki"
)

// componentDependencies holds the dependencies for control plane component as runtime configuration files.
type componentDependencies struct {
	CertTree            map[pki.RootCertName][]pki.LeafCertName
	CAFiles             []string
	KubeconfigFiles     []kubeconfig.File
	ExtraFileKeys       []string
	NeedsRootKubeconfig bool
}

var componentDepsRegistry = map[controlplanev1alpha1.OperationComponent]componentDependencies{
	controlplanev1alpha1.OperationComponentEtcd: {
		CertTree: map[pki.RootCertName][]pki.LeafCertName{
			pki.EtcdCACertName: {
				pki.EtcdServerCertName,
				pki.EtcdPeerCertName,
				pki.EtcdHealthcheckClientCertName,
				pki.ApiserverEtcdClientCertName,
			},
		},
		// TODO: remove this when we have a way to get the CA files from CertTree (pki.RootCertName)
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
}

func componentDeps(component controlplanev1alpha1.OperationComponent) componentDependencies {
	return componentDepsRegistry[component]
}

func (d componentDependencies) leafCertFiles() []pki.LeafCertName {
	if len(d.CertTree) == 0 {
		return nil
	}

	roots := make([]string, 0, len(d.CertTree))
	for root := range d.CertTree {
		roots = append(roots, string(root))
	}
	sort.Strings(roots)

	out := make([]pki.LeafCertName, 0)
	for _, root := range roots {
		out = append(out, d.CertTree[pki.RootCertName(root)]...)
	}
	return out
}
