package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/klog/v2"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"time"
)

func NewLinstorNodesReconciler(
	ctx context.Context,
// kClient kclient.Client,
	mgr manager.Manager,
	log logr.Logger,
	configSecretName string,
	interval int,
) error {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	configWatcher := make(chan string, 1)

	go func() {
		err := StartConfigWatcher(ctx, mgr, configWatcher, configSecretName)
		if err != nil {
			klog.Error(err)
			os.Exit(1)
		}
		// configWatcher <- "From watcher"
	}()
	for {
		select {
		case <-ticker.C:
			/*linstorNodes, err := GetLinstorNodes(ctx, #TODO)
			if err != nil {
				klog.Error(err)
				os.Exit(1)
			}*/
		case <-configWatcher:
			fmt.Printf("Start Watcher branch\n")

			fmt.Printf("End Watcher branch\n")
		case <-ctx.Done():
			return nil
		}
	}
}

func StartConfigWatcher(ctx context.Context, mgr manager.Manager, configWatcher chan<- string, configSecretName string) /*map[string]string,*/ error {
	// var linstorNodeSelector map[string]string
	// crd := &v1.CustomResourceDefinition{}
	// err := kClient.Get(ctx, kclient.ObjectKey{Name: "moduleconfigs.deckhouse.io"}, crd)
	// // schema := crd.Spec.Versions[0].Schema.OpenAPIV3Schema
	// myNode := &v12.Node{}
	// err = kClient.Get(ctx, kclient.ObjectKey{Name: "virtlab-az-0"}, myNode)
	// println("My node: %s", myNode.Name)
	// // kClient.Get(ctx, "ModuleConfig")
	// return linstorNodeSelector, err

}
