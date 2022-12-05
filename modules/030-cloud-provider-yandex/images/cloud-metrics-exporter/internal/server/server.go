// Copyright 2022 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"
	"exporter/internal/yandex"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi"
	log "github.com/sirupsen/logrus"
)

type Server struct {
	api             *yandex.CloudApi
	servicesForBach []string
	logger          *log.Entry

	router chi.Router
}

type serviceResult struct {
	metrics []byte
	service string
	err     error
}

func New(logger *log.Entry, api *yandex.CloudApi, servicesForBach []string) *Server {
	metricsSet := map[string]struct{}{}
	servicesList := make([]string, 0)
	for _, s := range servicesForBach {
		if _, ok := metricsSet[s]; !ok {
			metricsSet[s] = struct{}{}
			if api.HasService(s) {
				servicesList = append(servicesList, s)
			} else {
				logger.Warningf("incorrect service %s", s)
			}
		}
	}

	return &Server{
		logger:          logger,
		router:          chi.NewRouter(),
		api:             api,
		servicesForBach: servicesList,
	}
}

func (h *Server) Run(listenAddr string, stopCh chan struct{}) error {
	h.router.Route("/metrics", func(r chi.Router) {
		r.Get("/{service}", h.getByService)
		r.Get("/", h.getBatch)
	})

	h.router.Route("/healthz", func(r chi.Router) {
		r.Get("/", h.healthz)
	})

	srv := http.Server{
		Addr:         listenAddr,
		Handler:      h.router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 1 * time.Minute,
		IdleTimeout:  1 * time.Minute,
	}

	srv.RegisterOnShutdown(func() {
		close(stopCh)
	})

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		h.logger.Infof("Signal received: %v. Exiting...", <-signalChan)

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		err := srv.Shutdown(ctx)
		if err != nil {
			h.logger.Fatalf("Error occurred while closing the server: %v", err)
		}
		os.Exit(0)
	}()

	h.logger.Infof("Start listening on %q", listenAddr)

	return srv.ListenAndServe()
}

func (h *Server) healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("Ok")); err != nil {
		h.logger.Errorf("cannot write response for /healthz: %v", err)
	}
}

func (h *Server) writeError(upErr error, w http.ResponseWriter) {
	h.logger.Errorf("cannot scrape metrics: %v", upErr)
	w.WriteHeader(http.StatusInternalServerError)
	response := "Cannot scrape metrics. see server logs for describe error"

	if _, err := w.Write([]byte(response)); err != nil {
		h.logger.Errorf("cannot write response: %v", err)
	}
}

func (h *Server) getByService(w http.ResponseWriter, r *http.Request) {
	service := chi.URLParam(r, "service")
	h.logger.Infof("Request scrape metrics for service: %s", service)

	if !h.api.HasService(service) {
		h.writeError(fmt.Errorf("service '%s' not found", service), w)
		return
	}

	metrics, err := h.api.RequestMetrics(r.Context(), service)
	if err != nil {
		h.writeError(err, w)
		return
	}

	_, err = w.Write(metrics)
	if err != nil {
		h.logger.Errorf("cannot write response: %v", err)
	}
	h.logger.Infof("End request scrape metrics for service: %s", service)
}

func (h *Server) getBatch(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Request scrape batch metrics")
	servicesLen := len(h.servicesForBach)

	if servicesLen == 0 {
		h.writeError(fmt.Errorf("Pass services for scrape metrics"), w)
		return
	}

	resultsCh := make(chan *serviceResult, servicesLen)

	for _, s := range h.servicesForBach {
		go func(service string) {
			res := &serviceResult{
				service: service,
			}
			metrics, err := h.api.RequestMetrics(r.Context(), service)
			if err != nil {
				res.err = err
			} else {
				res.metrics = metrics
			}

			resultsCh <- res
		}(s)
	}

	var result []byte
	var servicesWithErrors []string

	for i := 0; i < servicesLen; i++ {
		res := <-resultsCh
		if res.err == nil {
			result = append(result, res.metrics...)
		} else {
			h.logger.Errorf("ERROR: Cannot get metrics for service %s: %v\n", res.service, res.err)
			servicesWithErrors = append(servicesWithErrors, res.service)
		}
	}

	if _, err := w.Write(result); err != nil {
		h.logger.Errorf("cannot write result: %v", err)
	}

	if len(servicesWithErrors) > 0 {
		h.logger.Warningf("End request scrape batch metrics with errors: %v", servicesWithErrors)
	} else {
		h.logger.Info("End request scrape batch metrics")
	}

}
