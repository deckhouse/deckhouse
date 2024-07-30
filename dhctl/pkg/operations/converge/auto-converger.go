// Copyright 2021 Flant JSC
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

package converge

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

type AutoConverger struct {
	runner        *converge.Runner
	checkInterval time.Duration
	listenAddress string
}

func NewAutoConverger(runner *converge.Runner, listenAddress string, interval time.Duration) *AutoConverger {
	return &AutoConverger{
		checkInterval: interval,
		listenAddress: listenAddress,
		runner:        runner,
	}
}

func (c *AutoConverger) Start() error {
	defer log.InfoLn("Stop autoconverger fully")

	log.InfoLn("Start exporter")
	log.InfoLn("Address: ", c.listenAddress)
	log.InfoLn("Checks interval: ", c.checkInterval)

	// channels to stop converge loop
	shutdownAllCh := make(chan struct{})
	doneCh := make(chan struct{})

	httpServer := c.getHTTPServer()

	tomb.RegisterOnShutdown("Stop http server and auto-converger loop", func() {
		close(shutdownAllCh)
		<-doneCh

		err := httpServer.Shutdown(context.TODO())
		if err != nil {
			log.ErrorF("Cannot shutdown http server %v", err)
		}
	})

	go c.convergerLoop(shutdownAllCh, doneCh)

	err := httpServer.ListenAndServe()
	if err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (c *AutoConverger) convergerLoop(shutdownCh <-chan struct{}, doneCh chan<- struct{}) {
	c.runConverge()

	ticker := time.NewTicker(c.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cache.ClearTemporaryDirs()
			c.runConverge()
		case <-shutdownCh:
			doneCh <- struct{}{}
			return
		}
	}
}

func (c *AutoConverger) getHTTPServer() *http.Server {
	indexPageContent := fmt.Sprintf(`<html>
             <head><title>CandI Auto converge</title></head>
             <body>
             <h1>CandI Auto converge terrform state every %s</h1>
             </body>
             </html>`, c.checkInterval.String())

	router := http.NewServeMux()
	router.Handle("/metrics", promhttp.Handler())
	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(indexPageContent))
	})

	return &http.Server{Addr: c.listenAddress, Handler: router, ReadHeaderTimeout: 30 * time.Second}
}

func (c *AutoConverger) runConverge() {
	log.InfoLn("Start next converge")

	err := c.runner.RunConverge()
	if err != nil {
		log.ErrorF("Converge error: %v\n", err)
	}
}
