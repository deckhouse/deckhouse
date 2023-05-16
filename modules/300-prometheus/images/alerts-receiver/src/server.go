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
	"encoding/json"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"
)

type serverStruct struct {
	*http.Server
	isReady atomic.Bool
}

func (s *serverStruct) readinessHandler(w http.ResponseWriter, _ *http.Request) {
	if !s.isReady.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *serverStruct) livenessHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func newServer(host, port string) *serverStruct {
	return &serverStruct{
		Server: &http.Server{
			Addr: net.JoinHostPort(host, port),
		},
	}
}

func (s *serverStruct) setHandlers(config *configStruct, store *storeStruct) {
	http.HandleFunc("/healthz", s.livenessHandler)
	http.HandleFunc("/readyz", s.readinessHandler)
	http.HandleFunc("/api/v1/alerts", s.alertsHandler(config, store))
	http.HandleFunc("/api/v2/alerts", s.alertsHandler(config, store))
}

func (s *serverStruct) ready() {
	s.isReady.Store(true)
}

func (s *serverStruct) notReady() {
	s.isReady.Store(false)
}

func (s *serverStruct) alertsHandler(config *configStruct, store *storeStruct) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
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

			// skip adding alerts if alerts queue is full
			if len(store.memStore.alerts) == config.capacity {
				log.Infof("cannot add alert to queue (capacity = %d), queue is full", config.capacity)
				s.notReady()
				continue
			}

			s.ready()
			store.memStore.insertAlert(alert)
		}
		w.WriteHeader(http.StatusOK)
	}
}
