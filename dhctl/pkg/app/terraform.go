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

	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	UseStateCacheAsk = "ask"
	UseStateCacheYes = "yes"
	UseStateCacheNo  = "no"
)

var (
	CacheDir   = defaultTmpAndStateDir
	UseTfCache = "ask"

	DropCache = false

	CacheKubeConfig          = ""
	CacheKubeConfigContext   = ""
	CacheKubeConfigInCluster = false
	CacheKubeNamespace       = ""
	CacheKubeName            = ""
	CacheKubeLabels          = make(map[string]string)

	ResourceManagementTimeout = ""
)

func DefineCacheFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("cache-dir", "Directory to store the cache.").
		Envar(configEnvName("CACHE_DIR")).
		StringVar(&CacheDir)

	cmd.Flag("use-cache", fmt.Sprintf(`Behaviour for using infrastructure state cache. May be:
	%s - ask user about it (Default)
   	%s - use cache
	%s  - don't use cache
	`, UseStateCacheAsk, UseStateCacheYes, UseStateCacheNo)).
		Envar(configEnvName("USE_CACHE")).
		Default(UseStateCacheAsk).
		EnumVar(&UseTfCache, UseStateCacheAsk, UseStateCacheYes, UseStateCacheNo)

	cmd.Flag("kube-cache-store-kubeconfig", "Path to kubernetes config file for storing cache in kubernetes secret").
		Envar(configEnvName("CACHE_STORE_KUBE_CONFIG")).
		StringVar(&CacheKubeConfig)
	cmd.Flag("kube-cachestore-kubeconfig-context", "Context from kubernetes config to connect to Kubernetes API. for storing cache in kubernetes secret").
		Envar(configEnvName("CACHE_STORE_KUBE_CONFIG_CONTEXT")).
		StringVar(&CacheKubeConfigContext)
	cmd.Flag("kube-cachestore-kube-client-from-cluster", "Use in-cluster Kubernetes API access. for storing cache in kubernetes secret").
		Envar(configEnvName("CACHE_STORE_KUBE_CLIENT_FROM_CLUSTER")).
		BoolVar(&CacheKubeConfigInCluster)
	cmd.Flag("kube-cachestore-namespace", "Use in-cluster Kubernetes API access. for storing cache in kubernetes secret").
		Envar(configEnvName("CACHE_STORE_KUBE_NAMESPACE")).
		StringVar(&CacheKubeNamespace)
	cmd.Flag("kube-cachestore-labels", "List labels for cache secrets").
		Envar(configEnvName("CACHE_STORE_KUBE_LABELS")).
		StringMapVar(&CacheKubeLabels)
	cmd.Flag("kube-cachestore-name", "Name for cache secret").
		Envar(configEnvName("CACHE_STORE_KUBE_NAME")).
		StringVar(&CacheKubeName)
}

func DefineDropCacheFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("yes-i-want-to-drop-cache", "All cached information will be deleted from your local cache.").
		Default("false").
		BoolVar(&DropCache)
}

func DefineTFResourceManagementTimeout(cmd *kingpin.CmdClause) {
	cmd.Flag("tf-resource-management-timeout", "Redefine infrastructure resource management timeouts").
		Envar(configEnvName("DHCTL_TF_RESOURCE_MANAGEMENT_TIMEOUT")).
		StringVar(&ResourceManagementTimeout)
}

func SetCacheDir(dir string) {
	CacheDir = dir
}

func GetCacheDir() string {
	return CacheDir
}

func GetDefaultCacheDir() string {
	return defaultTmpAndStateDir
}
