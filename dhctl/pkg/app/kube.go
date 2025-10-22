// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	KubeConfig        = ""
	KubeConfigContext = ""

	KubeConfigInCluster = false
)

func DefineKubeFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("kubeconfig", "Path to kubernetes config file.").
		Envar(configEnvName("KUBE_CONFIG")).
		StringVar(&KubeConfig)
	cmd.Flag("kubeconfig-context", "Context from kubernetes config to connect to Kubernetes API.").
		Envar(configEnvName("KUBE_CONFIG_CONTEXT")).
		StringVar(&KubeConfigContext)
	cmd.Flag("kube-client-from-cluster", "Use in-cluster Kubernetes API access.").
		Envar(configEnvName("KUBE_CLIENT_FROM_CLUSTER")).
		BoolVar(&KubeConfigInCluster)
}
