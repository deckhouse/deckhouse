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
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"
)

var (
	config     *Config
	alertStore *AlertStore
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})

	config = NewConfig()

	log.SetLevel(config.LogLevel)

	alertStore = NewStore(config.AlertsQueueLen)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		log.Infof("got signal %v", sig)
		cancel()
	}()

	http.HandleFunc("/healthz", readyHandler)
	http.HandleFunc("/", webhookHandler)

	srv := &http.Server{
		Addr: net.JoinHostPort(config.ListenHost, config.ListenPort),
	}

	go func() {
		err := srv.ListenAndServe()
		cancel()
		if err == nil || err == http.ErrServerClosed {
			return
		}
		log.Error(err)
	}()

	go reconcileLoop(ctx)

	<-ctx.Done()

	err := srv.Shutdown(context.Background())
	if err != nil && err != http.ErrServerClosed {
		log.Error(err)
	}
}

func readyHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	data := template.Data{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	for _, alert := range data.Alerts {
		if config.LogLevel == log.DebugLevel {
			a, err := json.Marshal(alert)
			if err != nil {
				log.Error(err)
				continue
			}
			log.Debugf("received alert: %s", a)
		}

		// remove resolved alerts
		if alert.Status == string(model.AlertResolved) {
			alertStore.Remove(&alert)
			continue
		}

		// skip DeadMansSwitch alerts
		if alert.Labels["alertname"] == "DeadMansSwitch" {
			log.Debug("skip DeadMansSwitch alert")
			continue
		}

		// skip adding alerts if alerts queue is full
		if len(alertStore.Alerts) == alertStore.length {
			log.Infof("cannot add alert to queue (max length = %d), queue is full", alertStore.length)
			continue
		}

		// update alert
		if _, ok := alertStore.Alerts[alert.Fingerprint]; ok {
			alertStore.Update(&alert)
			continue
		}

		// add alert
		alertStore.Add(&alert)
	}
	w.WriteHeader(http.StatusOK)
}

func reconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(reconcileTime)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reconcile()
		}
	}
}

func reconcile() {
	alertStore.m.RLock()
	defer alertStore.m.RUnlock()
	for _, v := range alertStore.Alerts {
		f := v.Alert.Fingerprint
		// remove outdated alerts
		if time.Since(v.LastReceivedTime) > 2*reconcileTime {
			alertStore.Remove(v.Alert)
			if err := alertStore.RemoveEvent(f); err != nil {
				log.Error(err)
			}
			continue
		}

		if _, ok := alertStore.Events[f]; ok {
			if err := alertStore.UpdateEvent(f); err != nil {
				log.Error(err)
			}
			continue
		}

		if err := alertStore.CreateEvent(f); err != nil {
			log.Error(err)
		}
	}

	// remove events which do not have corresponding alert entries
	for k := range alertStore.Events {
		if _, ok := alertStore.Alerts[k]; ok {
			continue
		}
		if err := alertStore.RemoveEvent(k); err != nil {
			log.Error(err)
		}
	}
}
