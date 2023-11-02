package main

import (
	"context"
	"encoding/base64"
	"os"
	"registry-modules-watcher/internal/backends"
	registryscaner "registry-modules-watcher/internal/backends/pkg/registry-scaner"
	"registry-modules-watcher/internal/backends/pkg/sender"
	"registry-modules-watcher/internal/wather"
	registryclient "registry-modules-watcher/pkg/registry-client"

	"k8s.io/klog"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
)

func main() {
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
	// TODO for range watchRegistries {}
	// TODO: remove b64 encode
	client, err := registryclient.NewClient("registry.deckhouse.io/deckhouse/fe/modules",
		registryclient.WithAuth(base64.RawStdEncoding.EncodeToString([]byte(regsecretRaw))),
	)
	if err != nil {
		klog.Fatal(err)
	}

	registryScaner := registryscaner.New(client)
	registryScaner.Subscribe(ctx)

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
