package main

import (
	"context"
	"os"

	sh_app "github.com/flant/shell-operator/pkg/app"
	utils_signal "github.com/flant/shell-operator/pkg/utils/signal"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"d8.io/upmeter/pkg/agent"
	"d8.io/upmeter/pkg/app"
	"d8.io/upmeter/pkg/probe/util"
	"d8.io/upmeter/pkg/server"
)

func main() {
	app.InitAppEnv()

	kpApp := kingpin.New("upmeter", "upmeter")

	// Server

	serverCommand := kpApp.Command("start", "Start upmeter informer")
	originsCount := serverCommand.Flag("origins", "The expected number of origins, used for exporting episodes as metrics when they are fulfilled by this number of agents.").
		Required().
		Int()

	serverCommand.Action(func(c *kingpin.ParseContext) error {
		sh_app.SetupLogging()
		log.Info("Starting upmeter server")

		srv := server.New(*originsCount)
		ctx, cancel := context.WithCancel(context.Background())

		err := srv.Start(ctx)
		if err != nil {
			cancel()
			log.Fatalf("cannot start server: %v", err)
		}

		// Block action by waiting signals from OS.
		utils_signal.WaitForProcessInterruption(func() {
			// FIXME the shutdown is still not graceful
			cancel()
			os.Exit(1)
		})

		return nil
	})

	sh_app.DefineKubeClientFlags(serverCommand)
	sh_app.DefineLoggingFlags(serverCommand)

	// Agent

	agentCommand := kpApp.Command("agent", "Start upmeter agent")

	agentCommand.Action(func(c *kingpin.ParseContext) error {
		sh_app.SetupLogging()
		log.Infof("Starting upmeter agent. ID=%s", util.AgentUniqueId())

		ctx, cancel := context.WithCancel(context.Background())

		a := agent.New()
		err := a.Start(ctx)
		if err != nil {
			cancel()
			log.Fatalf("cannot start agent: %v", err)
		}

		// Block 'main' by waiting signals from OS.
		utils_signal.WaitForProcessInterruption(func() {
			// FIXME the shutdown is still not graceful
			cancel()
			os.Exit(1)
		})
		return nil
	})

	sh_app.DefineKubeClientFlags(agentCommand)
	sh_app.DefineLoggingFlags(agentCommand)

	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}
