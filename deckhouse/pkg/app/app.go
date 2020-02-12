package app

import (
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
)

var AppName = "deckhouse"
var AppDescription = ""

var PodName = ""
var ContainerName = "deckhouse"

var FeatureWatchRegistry = "yes"
var InsecureRegistry = "no"
var SkipTlsVerifyRegistry = "no"
var RegistrySecretPath = "/etc/registrysecret"
var RegistryErrorsMaxTimeBeforeRestart = time.Hour

// TODO move to shell-operator
var KubeClientQpsDefault = "20"
var KubeClientQps float32 = 0.0
var KubeClientBurstDefault = "40"
var KubeClientBurst int = 0

func DefineStartCommandFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("pod-name", "Pod name to get image digest.").
		Envar("DECKHOUSE_POD").
		Required().
		StringVar(&PodName)
	cmd.Flag("feature-watch-registry", "Enable docker registry watcher (yes|no).").
		Envar("DECKHOUSE_WATCH_REGISTRY").
		Default(FeatureWatchRegistry).
		StringVar(&FeatureWatchRegistry)
	cmd.Flag("insecure-registry", "Use http to access registry (yes|no).").
		Envar("DECKHOUSE_INSECURE_REGISTRY").
		Default(InsecureRegistry).
		StringVar(&InsecureRegistry)
	cmd.Flag("skip-tls-verify-registry", "Trust self signed certificate of registry (yes|no).").
		Envar("DECKHOUSE_SKIP_TLS_VERIFY_REGISTRY").
		Default(SkipTlsVerifyRegistry).
		StringVar(&SkipTlsVerifyRegistry)

	// TODO move to shell-operator
	// Rate limit settings for kube client
	cmd.Flag("kube-client-qps", "QPS for kubeclient rest client").
		Envar("KUBE_CLIENT_QPS").
		Default(KubeClientQpsDefault).
		Float32Var(&KubeClientQps)
	cmd.Flag("kube-client-burst", "Burst for kubeclient rest client").
		Envar("KUBE_CLIENT_BURST").
		Default(KubeClientBurstDefault).
		IntVar(&KubeClientBurst)

}
