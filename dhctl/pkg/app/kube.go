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
	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// DefineKubeFlags registers Kubernetes-API flags, writing into o.
func DefineKubeFlags(cmd *cobra.Command, o *options.KubeOptions) {
	flags := cmd.Flags()
	StringFlag(flags, Flag{
		Name: "kubeconfig",
		Env:  "DHCTL_CLI_KUBE_CONFIG",
		Help: "Path to kubernetes config file.",
	}, &o.Config)
	StringFlag(flags, Flag{
		Name: "kubeconfig-context",
		Env:  "DHCTL_CLI_KUBE_CONFIG_CONTEXT",
		Help: "Context from kubernetes config to connect to Kubernetes API.",
	}, &o.ConfigContext)
	BoolFlag(flags, Flag{
		Name: "kube-client-from-cluster",
		Env:  "DHCTL_CLI_KUBE_CLIENT_FROM_CLUSTER",
		Help: "Use in-cluster Kubernetes API access.",
	}, &o.InCluster)
}
