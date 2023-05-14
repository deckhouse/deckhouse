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

	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	config     *configStruct
	alertStore *alertStoreStruct
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})

	config = newConfig()

	log.SetLevel(config.logLevel)

	alertStore = newStore(config.alertsQueueCapacity)
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
	http.HandleFunc("/api/v1/alerts", alertsHandler)
	http.HandleFunc("/api/v2/alerts", alertsHandler)

	srv := &http.Server{
		Addr: net.JoinHostPort(config.listenHost, config.listenPort),
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

func alertsHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var data model.Alerts
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	for _, alert := range data {
		if config.logLevel == log.DebugLevel {
			a, err := json.Marshal(alert)
			if err != nil {
				log.Error(err)
				continue
			}
			log.Debugf("received alert: %s", a)
		}
		/*
			// skip DeadMansSwitch alerts
			if alert.Labels["alertname"] == "DeadMansSwitch" {
				log.Debug("skip DeadMansSwitch alert")
				continue
			}
		*/
		// skip adding alerts if alerts queue is full
		if len(alertStore.alerts) == alertStore.capacity {
			log.Infof("cannot add alert to queue (capacity = %d), queue is full", alertStore.capacity)
			continue
		}

		alertStore.insertAlert(alert)
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
	log.Info("starting reconcile")

	crSet, err := listCRs()
	if err != nil {
		log.Error(err)
		return
	}

	// Add or update CRs
	alertSet := make(map[string]struct{}, len(alertStore.alerts))
	for f, v := range alertStore.alerts {
		if v.Resolved() {
			alertStore.removeAlert(f)
			continue
		}

		alertSet[f.String()] = struct{}{}

		// is alerts CR does not exist in cluster
		if _, ok := crSet[f.String()]; !ok {
			err := alertStore.insertCR(f)
			if err != nil {
				log.Error(err)
			}
		}

		// Update CR status
		err := alertStore.updateCRStatus(f)
		if err != nil {
			log.Error(err)
		}
	}

	// Remove CRs which do not have corresponding alerts
	for k := range crSet {
		if _, ok := alertSet[k]; !ok {
			err := removeCR(k)
			if err != nil {
				log.Error(err)
			}
		}
	}
}

/*
if alert.EndsAt.After(time.Now()) {
			api.m.Firing().Inc()
		} else {
			api.m.Resolved().Inc()
		}
*/

func listCRs() (map[string]struct{}, error) {
	log.Info("list CRs")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	crList, err := config.k8sClient.Resource(GVR).List(ctx, v1.ListOptions{
		LabelSelector:        "app=" + appName+ ",heritage=deckhouse",
		ResourceVersionMatch: v1.ResourceVersionMatchNotOlderThan,
		ResourceVersion:      "0",
	})
	cancel()
	if err != nil {
		return nil, err
	}
	res := make(map[string]struct{}, len(crList.Items))
	for _, item := range crList.Items {
		res[item.GetName()] = struct{}{}
	}
	log.Infof("found %d CRs in cluster", len(crList.Items))
	return res, nil
}

// Remove CR from cluster
func removeCR(fingerprint string) error {
	log.Infof("remove CR with name %s from cluster", fingerprint)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err := config.k8sClient.Resource(GVR).Delete(ctx, fingerprint, v1.DeleteOptions{})
	cancel()
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}
