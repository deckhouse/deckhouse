// Copyright 2023 Flant JSC
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

package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/flant/docs-builder/internal/docs"
	v1 "github.com/flant/docs-builder/internal/http/v1"
	"github.com/flant/docs-builder/pkg/k8s"
	"golang.org/x/sync/errgroup"
)

// flags
var (
	listenAddress    string
	src              string
	dst              string
	highAvailability bool
)

func init() {
	flag.StringVar(&listenAddress, "address", ":8081", "Address to listen on")
	flag.StringVar(&src, "src", "/app/hugo/", "Directory to load source files")
	flag.StringVar(&dst, "dst", "/mount/", "Directory for site files")
	flag.BoolVar(&highAvailability, "highAvailability", false, "high availability mod")
}

func main() {
	flag.Parse()

	ctx, stopNotify := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopNotify()

	logger := log.NewLogger(log.Options{
		Level: log.LogLevelFromStr(os.Getenv("LOG_LEVEL")).Level(),
	})

	lManager, err := k8s.NewLeasesManager(logger)
	if err != nil {
		logger.Fatalf("new leases manager: %s", err)
	}

	h := v1.NewHandler(docs.NewService(src, dst, highAvailability, logger), logger.Named("v1"))

	srv := &http.Server{
		Addr:    listenAddress,
		Handler: h,
	}

	eg, ctx := errgroup.WithContext(ctx)

	logger.Info("starting application")

	eg.Go(srv.ListenAndServe)
	eg.Go(lManager.Run(ctx))

	logger.Info("application started")

	<-ctx.Done()

	logger.Info("stopping application")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = lManager.Remove(ctx)
	if err != nil {
		logger.Error("lease removing failed", log.Err(err))
	}

	err = srv.Shutdown(ctx)
	if err != nil {
		logger.Error("shutdown failed", log.Err(err))
	}

	err = eg.Wait()
	if err != nil {
		logger.Error("error due stopping application", log.Err(err))
	}

	logger.Info("application stopped")
}
