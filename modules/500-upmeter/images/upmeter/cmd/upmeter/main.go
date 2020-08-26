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
	"upmeter/pkg/upmeter"
)

func main() {
	app.InitAppEnv()

	kpApp := kingpin.New("upmeter", "upmeter")

	// Informer part
	upCmd := kpApp.Command("start", "Start upmeter informer").
		Action(func(c *kingpin.ParseContext) error {
			sh_app.SetupLogging()
			log.Info("Start upmeter informer")
			informer := upmeter.NewDefaultInformer(context.Background())
			err := informer.Start()
			if err != nil {
				os.Exit(1)
			}
			// Block action by waiting signals from OS.
			utils_signal.WaitForProcessInterruption(func() {
				informer.Stop()
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
			log.Info("Start upmeter agent")
			upmeterAgent := agent.NewDefaultAgent(context.Background())
			err := upmeterAgent.Start()
			if err != nil {
				os.Exit(1)
			}
			// Block 'main' by waiting signals from OS.
			utils_signal.WaitForProcessInterruption(func() {
				upmeterAgent.Stop()
				os.Exit(1)
			})
			return nil
		})
	sh_app.DefineKubeClientFlags(agCmd)
	sh_app.DefineLoggingFlags(agCmd)

	kingpin.MustParse(kpApp.Parse(os.Args[1:]))
}
