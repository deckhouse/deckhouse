/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package start

import (
	log "github.com/sirupsen/logrus"
	"net/http"
	"system-registry-manager/internal/config"
)

var (
	server                     *http.Server
	controlPlaneManagerIsReady = false
)

func Start() {
	log.Info("Start service")
	log.Infof("Config file: %s", config.GetConfigFilePath())
	server = &http.Server{
		Addr: "127.0.0.1:8097",
	}
	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/readyz", readyzHandler)
	defer httpServerClose()

	controlPlaneManagerIsReady = true
	func() {
		err := server.ListenAndServe()
		if err == nil || err == http.ErrServerClosed {
			return
		}
		log.Error(err)
	}()
}

func httpServerClose() {
	if err := server.Close(); err != nil {
		log.Fatalf("HTTP close error: %v", err)
	}
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func readyzHandler(w http.ResponseWriter, _ *http.Request) {
	if controlPlaneManagerIsReady {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
}
