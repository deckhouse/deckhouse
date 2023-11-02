package main

import (
	"context"
	"flag"
	"os"
	"registry-modules-watcher/internal/backends"
	registryscaner "registry-modules-watcher/internal/backends/pkg/registry-scaner"
	"registry-modules-watcher/internal/backends/pkg/sender"
	"registry-modules-watcher/internal/wather"
	registryclient "registry-modules-watcher/pkg/registry-client"
	"strings"
	"time"

	"k8s.io/klog"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
)

func main() {
	registries := flag.String("watch-registries", "", "a list for followed registries")
	scanInterval := flag.Duration("scan-interval", 15*time.Minute, "interval for scanning the images. default 15 minutes")
	flag.Parse()

	if *registries == "" {
		klog.Fatal("watch-registries is empty")
	}

	ctx := context.Background()

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
		klog.Infof("Watch modules source: %q", registry)
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
	// use the current context in kubeconfig
	// config, err := clientcmd.BuildConfigFromFlags("", "/Users/dkoba/.kube/config")
	// if err != nil {
	// 	klog.Fatal(err)
	// }

	kClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	// * * * * * * * * *
	// Watch lease
	wather := wather.New(kClient)
	events, err := wather.Watch(context.TODO())
	if err != nil {
		klog.Fatal(err)
	}

	go func() {
		for event := range events {
			backends.Add(event)
		}
	}()

	<-ctx.Done()
}
