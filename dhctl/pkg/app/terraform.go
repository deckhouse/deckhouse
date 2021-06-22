package app

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	UseStateCacheAsk = "ask"
	UseStateCacheYes = "yes"
	UseStateCacheNo  = "no"
)

var (
	CacheDir   = filepath.Join(os.TempDir(), "dhctl")
	UseTfCache = "ask"

	DropCache = false

	CacheKubeConfig          = ""
	CacheKubeConfigContext   = ""
	CacheKubeConfigInCluster = false
	CacheKubeNamespace       = ""
	CacheKubeName            = ""
	CacheKubeLabels          = make(map[string]string)
)

func DefineCacheFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("cache-dir", "Directory to store the cache.").
		Envar(configEnvName("CACHE_DIR")).
		StringVar(&CacheDir)

	cmd.Flag("use-cache", fmt.Sprintf(`Behaviour for using terraform state cache. May be:
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
