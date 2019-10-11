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

func SetupGlobalSettings(kpApp *kingpin.Application) {
	kpApp.Flag("pod-name", "Pod name to get image digest.").
		Envar("DECKHOUSE_POD").
		Required().
		StringVar(&PodName)

	kpApp.Flag("feature-watch-registry", "Enable docker registry watcher (yes|no).").
		Envar("DECKHOUSE_WATCH_REGISTRY").
		Default(FeatureWatchRegistry).
		StringVar(&FeatureWatchRegistry)
	kpApp.Flag("insecure-registry", "Use http to access registry (yes|no).").
		Envar("DECKHOUSE_INSECURE_REGISTRY").
		Default(InsecureRegistry).
		StringVar(&InsecureRegistry)
	kpApp.Flag("skip-tls-verify-registry", "Trust self signed certificate of registry (yes|no).").
		Envar("DECKHOUSE_SKIP_TLS_VERIFY_REGISTRY").
		Default(SkipTlsVerifyRegistry).
		StringVar(&SkipTlsVerifyRegistry)
}
