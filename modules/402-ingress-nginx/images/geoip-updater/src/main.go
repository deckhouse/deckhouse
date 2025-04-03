/*
Copyright 2025 Flant JSC

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
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	// Parse command line arguments
	address := flag.String("address", ":8789", "Server address in format host:port")
	databasePath := flag.String("databasePath", "/chroot/etc/ingress-controller/geoip/", "Path to the GeoIP database")
	licenseKey := flag.String("licenseKey", "", "License key")
	accountID := flag.Int("accountID", 0, "Account ID")
	editionIDString := flag.String("editionIDs", "", "Editions names separated by comma")
	interval := flag.Int("interval", 1440, "Databases update check interval in minutes")
	flag.Parse()

	srv := NewHealthcheckServer(*address)

	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("Server error: %v\n", err)
		}
	}()

	// Create shutdown context with timeout
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	editionIDs := strings.Split(*editionIDString, ",")

	updater := NewUpdater(*interval, *licenseKey, *databasePath, *accountID, editionIDs)
	wg := sync.WaitGroup{}
	updater.Run(ctx, &wg)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until signal is received
	sig := <-sigChan
	log.Printf("Received signal: %v\n", sig)

	ctxTimeout, cancelFunc := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelFunc()

	// Initiate graceful shutdown
	if err := srv.Shutdown(ctxTimeout); err != nil {
		log.Printf("Healthcheck-server shutdown error: %v\n", err)
	}
	log.Println("Server shutdown complete")

	// Wait for updater to finish
	wg.Wait()
	log.Println("Updater shutdown complete")
}
