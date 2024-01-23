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
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})

	config := newConfig()
	store := newStore(config.capacity)

	log.SetLevel(config.logLevel)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		log.Infof("got signal %v", sig)
		cancel()
	}()

	server := newServer(config.listenHost, config.listenPort)
	server.setHandlers(config, store)

	go func() {
		err := server.ListenAndServe()
		if err == nil || err == http.ErrServerClosed {
			return
		}
		log.Error(err)
		cancel()
	}()

	server.setReadiness(true)

	go reconcileLoop(ctx, store)

	<-ctx.Done()

	err := server.Shutdown(context.Background())
	if err != nil && err != http.ErrServerClosed {
		log.Error(err)
	}
}

func reconcileLoop(ctx context.Context, s *storeStruct) {
	ticker := time.NewTicker(reconcileTime)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reconcile(ctx, s)
		}
	}
}

func reconcile(ctx context.Context, s *storeStruct) {
	log.Info("starting reconcile")

	crSet, err := s.clusterStore.listCRs(ctx)
	if err != nil {
		log.Error(err)
		return
	}

	// Add or update CRs
	alertSet := make(map[string]struct{}, len(s.memStore.alerts))
	s.memStore.RLock()
	for fingerprint, alert := range s.memStore.alerts {
		if alert.Resolved() {
			s.memStore.removeAlert(fingerprint)
			continue
		}

		alertSet[fingerprint.String()] = struct{}{}

		// is alerts CR does not exist in cluster, insert CR
		if _, ok := crSet[fingerprint.String()]; !ok {
			err := s.clusterStore.createCR(ctx, fingerprint.String(), alert)
			if err != nil {
				log.Error(err)
			}
		}

		// Update CR status
		err := s.clusterStore.updateCRStatus(ctx, fingerprint.String(), alert)
		if err != nil {
			log.Error(err)
		}
	}
	s.memStore.RUnlock()

	// Remove CRs which do not have corresponding alerts
	for k := range crSet {
		if _, ok := alertSet[k]; !ok {
			err := s.clusterStore.removeCR(ctx, k)
			if err != nil {
				log.Error(err)
			}
		}
	}
}
