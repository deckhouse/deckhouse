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
	"fmt"

	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// DefineCacheFlags registers infrastructure (terraform/tofu) state-cache flags into o.
func DefineCacheFlags(cmd *cobra.Command, o *options.CacheOptions) {
	flags := cmd.Flags()
	StringFlag(flags, Flag{
		Name: "cache-dir",
		Env:  "DHCTL_CLI_CACHE_DIR",
		Help: "Directory to store the cache.",
	}, &o.Dir)

	EnumFlag(flags, Flag{
		Name: "use-cache",
		Env:  "DHCTL_CLI_USE_CACHE",
		Help: fmt.Sprintf(`Behaviour for using infrastructure state cache. May be:
	%s - ask the user (Default)
   	%s - use cache
	%s  - don't use cache
	`, options.UseStateCacheAsk, options.UseStateCacheYes, options.UseStateCacheNo),
	}, &o.UseTfCache, options.UseStateCacheAsk, options.UseStateCacheYes, options.UseStateCacheNo)

	StringFlag(flags, Flag{
		Name: "kube-cache-store-kubeconfig",
		Env:  "DHCTL_CLI_CACHE_STORE_KUBE_CONFIG",
		Help: "Path to the kubernetes config file for storing the cache in a kubernetes secret",
	}, &o.KubeConfig)
	StringFlag(flags, Flag{
		Name: "kube-cachestore-kubeconfig-context",
		Env:  "DHCTL_CLI_CACHE_STORE_KUBE_CONFIG_CONTEXT",
		Help: "Context from the kubernetes config to connect to the Kubernetes API, for storing the cache in a kubernetes secret",
	}, &o.KubeConfigContext)
	BoolFlag(flags, Flag{
		Name: "kube-cachestore-kube-client-from-cluster",
		Env:  "DHCTL_CLI_CACHE_STORE_KUBE_CLIENT_FROM_CLUSTER",
		Help: "Use in-cluster Kubernetes API access, for storing the cache in a kubernetes secret",
	}, &o.KubeConfigInCluster)
	StringFlag(flags, Flag{
		Name: "kube-cachestore-namespace",
		Env:  "DHCTL_CLI_CACHE_STORE_KUBE_NAMESPACE",
		Help: "Namespace for storing the cache in a kubernetes secret",
	}, &o.KubeNamespace)
	PairsFlag(flags, Flag{
		Name: "kube-cachestore-labels",
		Env:  "DHCTL_CLI_CACHE_STORE_KUBE_LABELS",
		Help: "List of labels for cache secrets",
	}, &o.KubeLabels)
	StringFlag(flags, Flag{
		Name: "kube-cachestore-name",
		Env:  "DHCTL_CLI_CACHE_STORE_KUBE_NAME",
		Help: "Name of the cache secret",
	}, &o.KubeName)
}

// DefineDropCacheFlags registers --yes-i-want-to-drop-cache.
func DefineDropCacheFlags(cmd *cobra.Command, o *options.CacheOptions) {
	BoolFlag(cmd.Flags(), Flag{
		Name: "yes-i-want-to-drop-cache",
		Help: "All cached information will be deleted from your local cache.",
	}, &o.DropCache)
}

// DefineTFResourceManagementTimeout registers the infrastructure resource-management timeout override.
func DefineTFResourceManagementTimeout(cmd *cobra.Command, o *options.CacheOptions) {
	StringFlag(cmd.Flags(), Flag{
		Name: "tf-resource-management-timeout",
		Env:  "DHCTL_CLI_DHCTL_TF_RESOURCE_MANAGEMENT_TIMEOUT",
		Help: "Redefine infrastructure resource management timeouts",
	}, &o.ResourceManagementTimeout)
}
