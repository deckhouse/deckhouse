// Copyright 2024 Flant JSC
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
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/proxy"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"

	"registry-packages-proxy/internal/app"
	"registry-packages-proxy/internal/credentials"
)

func main() {
	config, err := app.InitFlags()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	logger := app.InitLogger(config)

	// init listener. Another listener is used in dhctl
	listener, err := net.Listen("tcp", config.ListenAddress)
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer listener.Close()

	bootstrapListener, err := net.Listen("tcp", config.RPPGetBinaryListenAddress)
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer bootstrapListener.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// init kube clients
	clientset, err := app.InitClient(config)
	if err != nil {
		logger.Fatal(err.Error())
	}

	dynamicClient, err := app.InitDynamicClient(config)
	if err != nil {
		logger.Fatal(err.Error())
	}

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", config.KubeConfig)
	if err != nil {
		logger.Fatal(err.Error())
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		logger.Fatal(err.Error())
	}
	if err := credentials.AddToScheme(scheme); err != nil {
		logger.Fatal(err.Error())
	}

	k8sClient, err := ctrlclient.New(kubeConfig, ctrlclient.Options{Scheme: scheme})
	if err != nil {
		logger.Fatal(err.Error())
	}

	// watch resources
	watcher := credentials.NewWatcher(clientset, dynamicClient, k8sClient, time.Hour, logger)
	go watcher.Watch(ctx)

	// init cache
	cache := app.NewCache(logger, config, app.RegisterMetrics())
	if !config.DisableCache {
		go cache.Reconcile(ctx)
	}

	registryClient := &registry.DefaultClient{}

	// init http server
	server := app.BuildServer()

	var opts []proxy.ProxyOption
	if !config.DisableCache {
		opts = append(opts, proxy.WithCache(cache))
	}
	// /v1/images/* CLI download routes are wired up by Proxy.Serve via ServeCLI and reach the
	// outside world through the kube-rbac-proxy sidecar on :4219, which authorizes them.
	rp := proxy.NewProxy(server, listener, watcher, logger, registryClient, opts...)
	rppGetServer := proxy.NewRPPClientBinaryServerFromRegistry(proxy.RPPClientBinaryServerOptions{
		Listener:           bootstrapListener,
		Logger:             logger,
		ClientConfigGetter: watcher,
		RegistryClient:     registryClient,
		SignCheck:          config.SignCheck,
		ClusterUUID:        config.ClusterUUID,
	})

	go rp.Serve(&proxy.Config{SignCheck: config.SignCheck})
	go rppGetServer.Serve()

	<-ctx.Done()

	rp.Stop()
	rppGetServer.Stop()
}
