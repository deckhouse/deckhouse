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
	"net/http"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
)

func main() {

	log.SetFormatter(&log.JSONFormatter{})

	var err error
	config, err = NewConfig()
	if err != nil {
		log.Fatal(err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
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
		log.Error(err)
	}()

	defer httpServerClose()

	removeOrphanFiles()

	runPhase(annotateNode())
	runPhase(waitNodeApproval())
	runPhase(waitImageHolderContainers())
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
	runPhase(upgrade())
	runPhase(config.writeLastAppliedConfigurationChecksum())

	cleanup()

	controlPlaneManagerIsReady = true
	// pause loop
	<-config.ExitChannel
}

func httpServerClose() {
	if err := server.Close(); err != nil {
		log.Fatalf("HTTP close error: %v", err)
	}
}
func runPhase(err error) {
	if err == nil {
		return
	}
	log.Error(err)
	cleanup()
	httpServerClose()
	os.Exit(1)
}
