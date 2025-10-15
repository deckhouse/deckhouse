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
	"github.com/fsnotify/fsnotify"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func main() {
	var err error
	config, err = NewConfig()
	if err != nil {
		log.Fatal(err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
		close(config.ExitChannel)
	}()

	server = &http.Server{
		Addr: "127.0.0.1:8095",
	}
	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/readyz", readyzHandler)

	go func() {
		err := server.ListenAndServe()
		if err == nil || err == http.ErrServerClosed {
			return
		}
		log.Error(err.Error())
	}()

	defer httpServerClose()

	removeOrphanFiles()

	runAllPhases(ctx)

	go watchConfigDir(configPath, func() {
		log.Info("Configuration changed — reloading phases...")
		runAllPhases(ctx)
	})

	controlPlaneManagerIsReady = true
	// pause loop
	<-config.ExitChannel
}

func httpServerClose() {
	if err := server.Close(); err != nil {
		log.Fatalf("HTTP close error: %v", err)
	}
}

func runAllPhases(ctx context.Context) {
	runPhase(DoAction(ctx, defaultBackoff, func(c context.Context) error {
		return annotateNode()
	}, "annotate node"))

	runPhase(DoAction(ctx, defaultBackoff, func(c context.Context) error {
		return waitNodeApproval()
	}, "wait for approval"))

	runPhase(DoAction(ctx, defaultBackoff, func(c context.Context) error {
		return waitImageHolderContainers()
	}, "wait for image holders"))

	runPhase(checkEtcdManifest())
	runPhase(checkKubeletConfig())
	runPhase(installKubeadmConfig())
	runPhase(installBasePKIfiles())
	runPhase(fillTmpDirWithPKIData())
	runPhase(renewCertificates())
	runPhase(renewKubeconfigs())
	runPhase(updateRootKubeconfig())
	runPhase(installExtraFiles())
	runPhase(convergeComponents())
	runPhase(config.writeLastAppliedConfigurationChecksum())

	cleanup()
}

func watchConfigDir(path string, onChange func()) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Errorf("failed to create watcher: %v", err)
		return
	}
	defer watcher.Close()

	err = watcher.Add(path)
	if err != nil {
		log.Errorf("failed to watch %s: %v", path, err)
		return
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
				log.Info("Config file change detected: %s (%s)", event.Name, event.Op.String())
				onChange()
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Errorf("watch error: %v", err)
		}
	}
}

func runPhase(err error) {
	if err == nil {
		return
	}
	log.Error(err.Error())
	cleanup()
	httpServerClose()
	os.Exit(1)
}
