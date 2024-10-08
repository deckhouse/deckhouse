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
	"os/signal"
	"syscall"
	"time"

	"github.com/flant/docs-builder/internal/docs"
	v1 "github.com/flant/docs-builder/internal/http/v1"
	"github.com/flant/docs-builder/pkg/k8s"
	"golang.org/x/sync/errgroup"

	"k8s.io/klog/v2"
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

	lManager, err := k8s.NewLeasesManager()
	if err != nil {
		klog.Fatalf("new leases manager: %s", err)
	}

	h := v1.NewHandler(docs.NewService(src, dst, highAvailability))

	srv := &http.Server{
		Addr:    listenAddress,
		Handler: h,
	}

	eg, ctx := errgroup.WithContext(ctx)

	klog.Info("starting application")

	eg.Go(srv.ListenAndServe)
	eg.Go(lManager.Run(ctx))

	klog.Info("application started")

	<-ctx.Done()

	klog.Info("stopping application")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = lManager.Remove(ctx)
	if err != nil {
		klog.Errorf("lease removing failed: %v", err)
	}

	err = srv.Shutdown(ctx)
	if err != nil {
		klog.Errorf("shutdown failed: %v", err)
	}

	err = eg.Wait()
	if err != nil {
		klog.Errorf("error due stopping application%v", err)
	}

	klog.Info("application stopped")
}
