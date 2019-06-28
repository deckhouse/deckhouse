package main

import (
	"flag"
	_ "net/http/pprof"
	"os"

	"github.com/romana/rlog"

	"github.com/flant/shell-operator/pkg/executor"
	utils_signal "github.com/flant/shell-operator/pkg/utils/signal"

	operator "github.com/flant/addon-operator/pkg/addon-operator"

	"github.com/deckhouse/deckhouse/antiopa/kube_helper"
	"github.com/deckhouse/deckhouse/antiopa/registry_watcher"
)

const DefaultMetricsPrefix = "antiopa_"

var AntiopaVersion = "dev"
var AddonOperatorVersion = "dev"
var ShellOperatorVersion = "dev"

// Get image digest from kube, start RegistryManager routine and imageChanged handler.
// Run addon-operator as a child process.
// No need to run executor.Reap here, because antiopa does not execute other commands.
// But addon-operator should start with forced Reaper.
func main() {
	// set flag.Parsed() for glog
	_ = flag.CommandLine.Parse([]string{})

	// Be a good parent - clean up after the child processes
	// in case if shell-operator is a PID1.
	go executor.Reap()

	operator.InitHttpServer()

	rlog.Infof("antiopa %s", AntiopaVersion)
	rlog.Infof("addon-operator %s, shell-operator %s", AddonOperatorVersion, ShellOperatorVersion)

	operator.MetricsPrefix = DefaultMetricsPrefix
	operator.ConfigMapName = "antiopa"
	operator.ValuesChecksumsAnnotation = "antiopa/values-checksums"

	// addon-operator init
	err := operator.Init()
	if err != nil {
		os.Exit(1)
	}

	// set kube client and namespace for kube_helper
	kube_helper.Init()

	// Init RegistryManager and start watcher
	err = registry_watcher.Init()
	if err != nil {
		os.Exit(1)
	}
	registry_watcher.Run()

	operator.Run()

	// Block action by waiting signals from OS.
	utils_signal.WaitForProcessInterruption()
}
