package main

import (
	"context"
	"encoding/json"
	"os"
	"watchdoc/internal/backends"
	registryscaner "watchdoc/internal/backends/pkg/registry-scaner"
	"watchdoc/internal/backends/pkg/sender"
	"watchdoc/internal/wather"
	registryclient "watchdoc/pkg/registry-client"

	v1 "k8s.io/api/core/v1"
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
	var regsecret v1.Secret
	regsecretRaw := os.Getenv("REGISTRY_AUTHS")
	if err := json.Unmarshal([]byte(regsecretRaw), &regsecret); err != nil {
		klog.Fatal(err)
	}

	dockerCfg := regsecret.Data["dockerconfigjson"]

	// * * * * * * * * *
	// Connect to registry
	// TODO for range watchRegistries {}
	client, err := registryclient.NewClient("registry.deckhouse.io/deckhouse/fe/modules",
		registryclient.WithAuth(string(dockerCfg)),
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
