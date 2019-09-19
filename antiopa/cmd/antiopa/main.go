package main

import (
	"flag"
	"fmt"
	_ "net/http/pprof"
	"os"

	"github.com/romana/rlog"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/antiopa/pkg/app"
	addon_operator_app "github.com/flant/addon-operator/pkg/app"
	shell_operator_app "github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/executor"
	utils_signal "github.com/flant/shell-operator/pkg/utils/signal"
)

// Variables with component versions. They set by 'go build' command.
var AntiopaVersion = "dev"
var AddonOperatorVersion = "dev"
var ShellOperatorVersion = "dev"

func main() {
	shell_operator_app.Version = ShellOperatorVersion
	addon_operator_app.Version = AddonOperatorVersion

	rlog.Infof("antiopa %s (shell-operator %s, addon-operator %s)", AntiopaVersion, ShellOperatorVersion, AddonOperatorVersion)

	kpApp := kingpin.New(app.AppName, fmt.Sprintf("%s %s: %s", app.AppName, AntiopaVersion, app.AppDescription))

	// global defaults
	app.SetupGlobalSettings(kpApp)
	// set global options for addon-operator
	addon_operator_app.SetupGlobalSettings(kpApp)

	// start main loop
	kpApp.Command("start", "Start antiopa.").
		Default().
		Action(func(c *kingpin.ParseContext) error {
			// Setting flag.Parsed() for glog.
			_ = flag.CommandLine.Parse([]string{})

			// Be a good parent - clean up after the child processes
			// in case if addon-operator is a PID 1 process.
			go executor.Reap()

			app.Start()

			// Block action by waiting signals from OS.
			utils_signal.WaitForProcessInterruption()

			return nil
		})

	kingpin.MustParse(kpApp.Parse(os.Args[1:]))

	return
}
