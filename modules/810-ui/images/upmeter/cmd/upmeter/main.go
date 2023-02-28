/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"d8.io/upmeter/pkg/agent"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/run"
	"d8.io/upmeter/pkg/server"
)

func main() {
	var (
		loggerConfig = &loggerConfig{}

		kubeConfig  = &kubernetes.Config{}
		agentConfig = agent.NewConfig()

		serverConfig = server.NewConfig()
	)

	app := kingpin.New("upmeter", "upmeter")
	logger := log.StandardLogger()

	// Server

	serverCommand := app.Command("start", "Start upmeter server")
	parseServerArgs(serverCommand, serverConfig)
	parseKubeArgs(serverCommand, kubeConfig)
	parseLoggerArgs(serverCommand, loggerConfig)
	serverCommand.Action(func(c *kingpin.ParseContext) error {
		setupLogger(logger, loggerConfig)

		logger.Info("Starting upmeter server")
		logger.Debugf("Logger config: %v", loggerConfig)
		logger.Debugf("Server config: %v", serverConfig)

		srv := server.New(serverConfig, kubeConfig, logger)
		startCtx, cancelStart := context.WithCancel(context.Background())

		go func() {
			defer cancelStart()

			err := srv.Start(startCtx)
			if err != nil {
				logger.Fatalf("cannot start server: %v", err)
			}
		}()

		// Blocks waiting signals from OS.
		shutdown(func() {
			cancelStart()

			err := srv.Stop()
			if err != nil {
				logger.Fatalf("error stop server gracefully: %v", err)
			}

			os.Exit(0)
		})

		return nil
	})

	// Agent

	agentCommand := app.Command("agent", "Start upmeter agent")
	parseKubeArgs(agentCommand, kubeConfig)
	parseAgentArgs(agentCommand, agentConfig)
	parseLoggerArgs(agentCommand, loggerConfig)
	agentCommand.Action(func(c *kingpin.ParseContext) error {
		setupLogger(logger, loggerConfig)
		logger.Infof("Starting upmeter agent. ID=%s", run.ID())
		logger.Debugf("Logger config: %v", loggerConfig)
		logger.Debugf("Agent config: %v", agentConfig)
		logger.Debugf("Kubernetes config: %v", kubeConfig)

		a := agent.New(agentConfig, kubeConfig, logger)

		startCtx, cancelStart := context.WithCancel(context.Background())

		go func() {
			defer cancelStart()

			err := a.Start(startCtx)
			if err != nil {
				cancelStart()
				logger.Fatalf("cannot start agent: %v", err)
			}
		}()

		// Blocks waiting signals from OS.
		shutdown(func() {
			cancelStart()

			err := a.Stop()
			if err != nil {
				logger.Fatalf("error stopp agent gracefully: %v", err)
			}

			os.Exit(0)
		})

		return nil
	})

	kingpin.MustParse(app.Parse(os.Args[1:]))
}

// shutdown waits for SIGINT or SIGTERM and runs a callback function.
//
// First signal start a callback function, which should call os.Exit(0).
// Next signal will force exit with os.Exit(128 + signalValue). If no cb is given, the exist is also forced.
func shutdown(cb func()) {
	exitGracefully := cb != nil

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	for {
		sig := <-ch

		if exitGracefully {
			exitGracefully = false
			log.Infof("Shutdown called with %q", sig.String())
			go cb()
			continue
		}

		log.Infof("Forced shutdown with %q", sig.String())
		signum := 0
		if v, ok := sig.(syscall.Signal); ok {
			signum = int(v)
		}
		os.Exit(128 + signum)
	}
}
