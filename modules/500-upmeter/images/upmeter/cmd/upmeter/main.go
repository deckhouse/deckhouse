package main

import (
	"context"
	"os"

	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	sh_app "github.com/flant/shell-operator/pkg/app"
	utils_signal "github.com/flant/shell-operator/pkg/utils/signal"

	"upmeter/pkg/agent"
	"upmeter/pkg/app"
	"upmeter/pkg/probe/util"
	"upmeter/pkg/upmeter"
)

func main() {
	app.InitAppEnv()

	kpApp := kingpin.New("upmeter", "upmeter")

	// Informer part
	upCmd := kpApp.Command("start", "Start upmeter informer")
	originsCount := upCmd.Flag("origins", "The expected number of origins, used for exporting episodes as metrics when they are fulfilled by this number of agents.").
		Required().
		Int()

	upCmd.Action(func(c *kingpin.ParseContext) error {
		sh_app.SetupLogging()
		log.Info("Starting upmeter informer")

		informer := upmeter.New(*originsCount)
		ctx, cancel := context.WithCancel(context.Background())

		err := informer.Start(ctx)
		if err != nil {
			cancel()
			log.Fatalf("cannot start informer: %v", err)
		}

		// Block action by waiting signals from OS.
		utils_signal.WaitForProcessInterruption(func() {
			// FIXME the shutdown is still not graceful
			cancel()
			os.Exit(1)
		})

		return nil
	})
	sh_app.DefineKubeClientFlags(upCmd)
	sh_app.DefineLoggingFlags(upCmd)

	// Agent part
	agCmd := kpApp.Command("agent", "Start upmeter agent").
		Action(func(c *kingpin.ParseContext) error {
			sh_app.SetupLogging()
			log.Infof("Starting upmeter agent. Id=%s", util.AgentUniqueId())

			ctx, cancel := context.WithCancel(context.Background())

			agent := agent.NewDefaultAgent()
			err := agent.Start(ctx)
			if err != nil {
				cancel()
				os.Exit(1)
			}

			// Block 'main' by waiting signals from OS.
			utils_signal.WaitForProcessInterruption(func() {
				// FIXME the shutdown is still not graceful
				cancel()
				os.Exit(1)
			})
			return nil
		})
	sh_app.DefineKubeClientFlags(agCmd)
	sh_app.DefineLoggingFlags(agCmd)

	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}
