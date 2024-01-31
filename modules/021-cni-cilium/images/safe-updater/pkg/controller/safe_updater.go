/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"safe-updater/config"
	"safe-updater/pkg/logger"
	"time"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	safeUpdaterCtrlName = "safe-updater-controller"
)

func SafeUpdaterController(
	ctx context.Context,
	mgr manager.Manager,
	cfg config.Options,
	log logger.Logger,
) (controller.Controller, error) {
	cl := mgr.GetClient()

	c, err := controller.New(safeUpdaterCtrlName, mgr, controller.Options{
		Reconciler: reconcile.Func(func(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
			return reconcile.Result{}, nil
		}),
	})

	if err != nil {
		log.Error(err, "[SafeUpdaterController] unable to create controller")
		return nil, err
	}

	go func() {
		CiliumAgentDS, err := GetDS(ctx, cl, "cilium", "agent")
		if err != nil {
			return nil, err
		}

		fmt.Printf("generation of DS")
		CiliumAgentPods, err := ListPods(ctx, cl, "cilium", "app=agent")
		if err != nil {
			return nil, err
		}

		CiliumAgentPodOnSameNode, err := GetPodOnSameNode(CiliumAgentPods, NODE_NAME)
		if err != nil {
			return nil, err
		}

		fmt.Printf("generation of pod on same node")

		if CiliumAgentDS.generation != CiliumAgentPodOnSameNode.generation {
			fmt.Printf("generation do not match. Deleting pod")
			_, err := DeletePod(ctx, cl, "cilium", CiliumAgentPodOnSameNode.Name)
			if err != nil {
				return nil, err
			}

			fmt.Printf("Pod deleted")
		}

		for {
			fmt.Printf("Wait until pod created on same node")
			NewCiliumAgentPodOnSameNode, err := GetPodOnSameNode(CiliumAgentPods, NODE_NAME)
			if err != nil {
				return nil, err
			}

			if NewCiliumAgentPodOnSameNode.Name != nil {
				fmt.Printf("Pod created")
				break
			}
			time.Sleep(cfg.StatusScanInterval * time.Second)
		}

		for {
			PodStatus, err := GetPodStatus(CiliumAgentPodOnSameNode)
			if err != nil {
				return nil, err
			}

			if PodStatus != Ready {
				fmt.Printf("Wait until pod become Ready")
			} else {
				break
			}
			time.Sleep(cfg.StatusScanInterval * time.Second)
		}

		// _, err := GetPod(ctx, cl, "cilium", "agent")
		// if err != nil {
		//		log.Error(err, "GetPod unable to get")
		// }

	}()

	return c, err
}

func GetDS() {}

func ListPods() {}

func GetPodOnSameNode() {}

func DeletePod() {}

func GetPodStatus() {}

func GetPod(ctx context.Context, cl client.Client, namespace, name string) (*v1.Pod, error) {

	pod := &v1.Pod{}
	err := cl.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, pod)
	if err != nil {
		return nil, err
	}
	return pod, nil
}
