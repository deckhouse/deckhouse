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
	scanInterval := flag.Float64("scan-interval", 30, "interval for scanning the images. default 15 minutes")
	flag.Parse()

	if *registries == "" {
		klog.Fatal("watch-registries is empty")
	}

	ctx := context.Background()

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
	// dockerconfigjson
	regsecretRaw := os.Getenv("REGISTRY_AUTHS")

	// * * * * * * * * *
	// Connect to registry
	// TODO: remove b64 encode
	clients := []registryscaner.Client{}
	for _, registry := range strings.Split(*registries, ",") {
		client, err := registryclient.NewClient(registry,
			registryclient.WithAuth(regsecretRaw),
		)
		if err != nil {
			klog.Fatal(err)
		}

		clients = append(clients, client)
	}

	registryScaner := registryscaner.New(clients...)
	registryScaner.Subscribe(ctx, time.Duration(*scanInterval*float64(time.Second)))

	// * * * * * * * * *
	// New sender
	sender := sender.New()

	// * * * * * * * * *
	// New backends service
	backends := backends.New(registryScaner, sender)

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
