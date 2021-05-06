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
