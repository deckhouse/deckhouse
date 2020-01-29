package main

import (
	"fmt"
	_ "net/http/pprof"
	"os"

	. "github.com/flant/libjq-go"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	sh_app "github.com/flant/shell-operator/pkg/app"
	sh_debug "github.com/flant/shell-operator/pkg/debug"
	"github.com/flant/shell-operator/pkg/executor"
	utils_signal "github.com/flant/shell-operator/pkg/utils/signal"

	ad_app "github.com/flant/addon-operator/pkg/app"

	"flant/deckhouse/pkg/app"
	"flant/deckhouse/pkg/deckhouse"
	"flant/deckhouse/pkg/helpers"
)

// Variables with component versions. They set by 'go build' command.
var DeckhouseVersion = "dev"
var AddonOperatorVersion = "dev"
var ShellOperatorVersion = "dev"

func main() {
	// TODO DELETE THIS AFTER MIGRATION
	// temporary fix to migrate from ANTIOPA_POD to DECKHOUSE_POD
	antiopaPod := os.Getenv("ANTIOPA_POD")
	if antiopaPod != "" {
		err := os.Setenv("DECKHOUSE_POD", antiopaPod)
		if err != nil {
			panic(err)
		}
	}
	// END DELETE THIS AFTER MIGRATION

	sh_app.Version = ShellOperatorVersion
	ad_app.Version = AddonOperatorVersion

	kpApp := kingpin.New(app.AppName, fmt.Sprintf("%s %s: %s", app.AppName, DeckhouseVersion, app.AppDescription))

	// override usage template to reveal additional commands with information about start command
	kpApp.UsageTemplate(sh_app.OperatorUsageTemplate(app.AppName))

	// print version
	kpApp.Command("version", "Show version.").Action(func(c *kingpin.ParseContext) error {
		fmt.Printf("deckhouse %s (addon-operator %s, shell-operator %s)", DeckhouseVersion, AddonOperatorVersion, ShellOperatorVersion)
		return nil
	})

	// start main loop
	startCmd := kpApp.Command("start", "Start deckhouse.").
		Default().
		Action(func(c *kingpin.ParseContext) error {
			sh_app.SetupLogging()
			log.Infof("deckhouse %s (addon-operator %s, shell-operator %s)", DeckhouseVersion, AddonOperatorVersion, ShellOperatorVersion)

			// Be a good parent - clean up after the child processes
			// in case if addon-operator is a PID 1 process.
			go executor.Reap()

			jqDone := make(chan struct{})
			go JqCallLoop(jqDone)

			operator := deckhouse.DefaultDeckhouse()
			err := deckhouse.InitAndStart(operator)
			if err != nil {
				os.Exit(1)
			}

			// Block action by waiting signals from OS.
			utils_signal.WaitForProcessInterruption(func() {
				operator.Stop()
			})

			return nil
		})
	// Set default log type as json
	sh_app.LogType = "json"
	ad_app.SetupStartCommandFlags(kpApp, startCmd)
	app.DefineStartCommandFlags(startCmd)

	// Add debug commands from shell-operator and addon-operator
	sh_debug.DefineDebugCommands(kpApp)
	ad_app.DefineDebugCommands(kpApp)

	helpers.DefineHelperCommands(kpApp)

	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
	return
}
