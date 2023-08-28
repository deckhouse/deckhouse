package controllers

import (
	"context"
	"github.com/go-logr/logr"
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/klog/v2"
	"os"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

func NewLinstorNodesReconciler(
	ctx context.Context,
	kClient kclient.Client,
	log logr.Logger,
	interval int,
) error {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)

	for {
		select {
		case <-ticker.C:
			_, err := GetLinstorNodeSelector(ctx, kClient)
			if err != nil {
				klog.Error(err)
				os.Exit(1)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func GetLinstorNodeSelector(ctx context.Context, kClient kclient.Client) (map[string]string, error) {
	var linstorNodeSelector map[string]string
	crd := &v1.CustomResourceDefinition{}
	err := kClient.Get(ctx, kclient.ObjectKey{Name: "moduleconfigs.deckhouse.io"}, crd)
	// schema := crd.Spec.Versions[0].Schema.OpenAPIV3Schema
	myNode := &v12.Node{}
	err = kClient.Get(ctx, kclient.ObjectKey{Name: "virtlab-az-0"}, myNode)
	println("My node: %s", myNode.Name)
	// kClient.Get(ctx, "ModuleConfig")
	return linstorNodeSelector, err
}
