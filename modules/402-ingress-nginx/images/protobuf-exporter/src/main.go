/*
Copyright 2021 Flant JSC

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
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/common/log"

	"github.com/flant/protobuf_exporter/pkg/server"
	"github.com/flant/protobuf_exporter/pkg/vault"
)

func main() {
	telemetryAddress := ":8080"
	exporterAddress := ":8081"
	mappingsPath := "./mappings.yaml"
	logLevel := "info"

	flag.StringVar(&telemetryAddress, "server.telemetry-address", telemetryAddress, "Address to listen telemetry messages")
	flag.StringVar(&exporterAddress, "server.exporter-address", exporterAddress, "Address to export prometheus metrics")
	flag.StringVar(&mappingsPath, "mappings", mappingsPath, "Path to mappings")
	flag.StringVar(&logLevel, "log-level", logLevel, "Log level")
	flag.Parse()

	if err := log.Base().SetLevel(logLevel); err != nil {
		log.Fatalf("Set log level: %v", err)
	}

	metricsVault := vault.NewVault()
	mappings, err := vault.LoadMappingsByPath(mappingsPath)
	if err != nil {
		log.Fatalf("Can't load mappings from %q failed: %v", mappingsPath, err)
	}

	err = metricsVault.RegisterMappings(mappings)
	if err != nil {
		log.Fatalf("Mappings registration from %q failed: %v", mappingsPath, err)
	}

	errorCh := make(chan error)
	metricsServer := server.NewMetricsServer()
	tcpServer := server.NewTelemetryServer(metricsVault)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	go tcpServer.Start(telemetryAddress, errorCh)
	go metricsServer.Start(exporterAddress, errorCh)

	tick := time.NewTicker(time.Second)
	for {
		select {
		// TODO: Think about deleting stale metrics on Collect instead of using scheduled job
		case <-tick.C:
			metricsVault.RemoveStaleMetrics()
		case s := <-signalChan:
			log.Warnf("Signal received: %v. Exiting...", s)
			tcpServer.Close()
			metricsServer.Close()
			tick.Stop()
			os.Exit(0)
		case e := <-errorCh:
			log.Errorf("Error received: %v", e)
			tick.Stop()
			os.Exit(1)
		}
	}
}
