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
	"errors"
	"flag"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/flant/docs-builder/pkg/k8s"

	"github.com/gorilla/mux"
	apierror "k8s.io/apimachinery/pkg/api/errors"
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

	h := newHandler(highAvailability)

	srv := &http.Server{
		Addr:    listenAddress,
		Handler: h,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			klog.Fatalf("listen: %s", err)
		}
	}()
	klog.Info("Server started")

	go func() {
		err = lManager.Run(ctx)
		if !errors.Is(err, context.Canceled) && err != nil {
			klog.Fatalf("run lease manager: %s", err)
		}
	}()

	<-ctx.Done()
	klog.Info("Server stopped")

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
}
