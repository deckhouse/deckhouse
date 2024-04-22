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
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/alertmanager/types"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})

	config := newConfig()
	store := newStore(config.capacity)

	log.SetLevel(config.logLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := newServer(config.listenHost, config.listenPort)
	server.setHandlers(config, store)

	log.Infof("starting listener: %s\n\n", server.Addr)
	go func() {
		err := server.ListenAndServe()
		if err == nil || err == http.ErrServerClosed {
			return
		}
		log.Error(err)
		stop()
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

	log.Info("remove resolved alerts")
	s.memStore.removeResolvedAlerts()

	// get deep copy
	alerts := s.memStore.deepCopy()

	if len(alerts) == s.memStore.capacity {
		addClusterHasTooManyAlertsAlert(alerts, s.memStore.capacity)
	}

	if time.Now().After(s.memStore.lastDMSReceived.Add(2 * reconcileTime)) {
		addMissingDeadMensSwitchAlert(alerts)
	}

	for fingerprint, alert := range alerts {
		// is alerts CR does not exist in cluster, insert CR
		if _, ok := crSet[fingerprint]; !ok {
			err := s.clusterStore.createCR(ctx, fingerprint, alert)
			if err != nil {
				log.Error(err)
			}
		} else {
			// Update CR status
			err := s.clusterStore.updateCRStatus(ctx, fingerprint, alert.StartsAt, alert.UpdatedAt)
			if err != nil {
				log.Error(err)
			}
		}
	}

	// Remove CRs which do not have corresponding alerts
	for k := range crSet {
		if _, ok := alerts[k]; !ok {
			err := s.clusterStore.removeCR(ctx, k)
			if err != nil {
				log.Error(err)
			}
		}
	}
	log.Info("finishing reconcile")
}

// generate queue fullness alert
func addClusterHasTooManyAlertsAlert(alerts map[string]*types.Alert, capacity int) {
	log.Info("add queue fullness alert")
	alert := generateAlert(ClusterHasTooManyAlertsAlertName, fmt.Sprintf("Cluster has more than %d active alerts.", capacity))
	alerts[strings.ToLower(ClusterHasTooManyAlertsAlertName)] = alert
}

// generate alert about missing deadmansswitch
func addMissingDeadMensSwitchAlert(alerts map[string]*types.Alert) {
	log.Infof("add missed %s alert", DMSAlertName)
	alert := generateAlert(MissingDMSAlertName, "Entire Alerting pipeline is not functional.")
	alerts[strings.ToLower(MissingDMSAlertName)] = alert
}
