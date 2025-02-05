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
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"registry-modules-watcher/internal/backends"
	registryscaner "registry-modules-watcher/internal/backends/pkg/registry-scaner"
	"registry-modules-watcher/internal/backends/pkg/sender"
	"registry-modules-watcher/internal/watcher"
	registryclient "registry-modules-watcher/pkg/registry-client"
)

/*
	klog.V(0).InfoS = klog.InfoS - Generally useful for this to always be visible to a cluster operator
	Programmer errors
	Logging extra info about a panic
	CLI argument handling
	klog.V(1).InfoS - A reasonable default log level if you don't want verbosity.
	Information about config (listening on X, watching Y)
	Errors that repeat frequently that relate to conditions that can be corrected (pod detected as unhealthy)
	klog.V(2).InfoS - Useful steady state information about the service and important log messages that may correlate to significant changes in the system. This is the recommended default log level for most systems.
	Logging HTTP requests and their exit code
	System state changing (killing pod)
	Controller state change events (starting pods)
	Scheduler log messages
	klog.V(3).InfoS - Extended information about changes
	More info about system state changes
	klog.V(4).InfoS - Debug level verbosity
	Logging in particularly thorny parts of code where you may want to come back later and check it
	klog.V(5).InfoS - Trace level verbosity
	Context to understand the steps leading up to errors and warnings
	More information for troubleshooting reported issues
*/

func main() {
	klog.InitFlags(nil)
	registries := flag.String("watch-registries", "", "a list for followed registries")
	scanInterval := flag.Duration("scan-interval", 15*time.Minute, "interval for scanning the images. default 15 minutes")
	flag.Parse()

	if *registries == "" {
		klog.Fatal("watch-registries is empty")
	}

	ctx, stopNotify := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopNotify()

	// * * * * * * * * *
	// dockerconfigjson
	regsecretRaw := os.Getenv("REGISTRY_AUTHS")
	if regsecretRaw == "" {
		klog.Fatal("registry auths not set")
	}

	// * * * * * * * * *
	// Connect to registry
	clients := make([]registryscaner.Client, 0)
	for _, registry := range strings.Split(*registries, ",") {
		klog.V(0).Infof("Watch modules source: %q", registry)
		client, err := registryclient.NewClient(registry,
			registryclient.WithAuth(regsecretRaw),
		)
		if err != nil {
			klog.Errorf("no dockercfg auth set for source: %q. Skipping", registry)
			continue
		}

		// TODO: some registry ping to check credentials
		clients = append(clients, client)
	}

	if len(clients) == 0 {
		klog.Fatal("no registries to watch")
	}

	registryScaner := registryscaner.New(clients...)
	registryScaner.Subscribe(ctx, *scanInterval)

	// * * * * * * * * *
	// New sender
	sender := sender.New()

	// * * * * * * * * *
	// New backends service
	backends := backends.New(registryScaner, sender)

	// * * * * * * * * *
	// Init kube client
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatal(err)
	}

	kClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	// * * * * * * * * *
	// Watch lease
	namespace := os.Getenv("POD_NAMESPACE")
	wather := watcher.New(kClient, namespace)
	wather.Watch(ctx, backends.Add, backends.Delete)

	<-ctx.Done()
}
