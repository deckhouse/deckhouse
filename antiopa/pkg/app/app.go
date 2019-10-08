package app

import (
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
)

var AppName = "antiopa"
var AppDescription = ""

var PodName = ""
var ContainerName = "antiopa"

var FeatureWatchRegistry = "yes"
var InsecureRegistry = "no"
var SkipTlsVerifyRegistry = "no"
var RegistrySecretPath = "/etc/registrysecret"
var RegistryErrorsMaxTimeBeforeRestart = time.Hour

func SetupGlobalSettings(kpApp *kingpin.Application) {
	kpApp.Flag("pod-name", "Pod name to init additional container with tiller.").
		Envar("ANTIOPA_POD").
		Required().
		StringVar(&PodName)

	kpApp.Flag("feature-watch-registry", "Enable docker registry watcher (yes|no).").
		Envar("ANTIOPA_WATCH_REGISTRY").
		Default(FeatureWatchRegistry).
		StringVar(&FeatureWatchRegistry)
	kpApp.Flag("insecure-registry", "Use http to access registry (yes|no).").
		Envar("ANTIOPA_INSECURE_REGISTRY").
		Default(InsecureRegistry).
		StringVar(&InsecureRegistry)
	kpApp.Flag("skip-tls-verify-registry", "Trust self signed certificate of registry (yes|no).").
		Envar("ANTIOPA_SKIP_TLS_VERIFY_REGISTRY").
		Default(SkipTlsVerifyRegistry).
		StringVar(&SkipTlsVerifyRegistry)
}
