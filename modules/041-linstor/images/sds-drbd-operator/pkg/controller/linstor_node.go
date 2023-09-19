package controller

import (
	"context"
	"fmt"
	lclient "github.com/LINBIT/golinstor/client"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"net"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	sdsapi "st2/api/v1alpha1"
	"time"
)

const (
	LinstorNodeControllerName = "linstor-node-controller"
	LinstorControllerType     = "CONTROLLER"
	LinstorSatelliteType      = "SATELLITE"
	LinstorOnlineStatus       = "ONLINE"
	LinstorOfflineStatus      = "OFFLINE"
	LinstorNodePort           = 3367  //
	LinstorEncryptionType     = "SSL" // "Plain"
	reachableTimeout          = 10 * time.Second
)

func NewLinstorNode(
	ctx context.Context,
	mgr manager.Manager,
	lc *lclient.Client,
	configSecretName string,
	interval int,
) (controller.Controller, error) {
	cl := mgr.GetClient()
	log := mgr.GetLogger()

	c, err := controller.New(LinstorNodeControllerName, mgr, controller.Options{
		Reconciler: reconcile.Func(func(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
			return reconcile.Result{}, nil
		}),
	})

	if err != nil {
		return nil, err
	}

	// err = c.Watch(
	// 	source.Kind(mgr.GetCache(), &v1.Node{}),
	// 	handler.Funcs{
	// 		CreateFunc: func(ctx context.Context, e event.CreateEvent, limitingInterface workqueue.RateLimitingInterface) {
	// 			log.Info("NODES: CREATE Event. NODE_NAME: " + e.Object.GetName())
	// 			for k, v := range e.Object.GetLabels() {
	// 				log.Info("NODES: CREATE Event. NODE_LABEL: " + k + ":" + v)
	// 			}
	//
	// 		},
	// 		UpdateFunc: func(ctx context.Context, e event.UpdateEvent, limitingInterface workqueue.RateLimitingInterface) {
	// 			log.Info("NODES: UPDATE Event. NEW NAME:" + e.ObjectNew.GetName())
	// 			for k, v := range e.ObjectNew.GetLabels() {
	// 				log.Info("NODES: CREATE Event. NEW NODE_LABEL: " + k + ":" + v)
	// 			}
	// 		},
	// 		DeleteFunc: func(ctx context.Context, e event.DeleteEvent, limitingInterface workqueue.RateLimitingInterface) {
	// 			log.Info("NODES: DELETE Event. NAME:" + e.Object.GetName())
	// 		},
	// 		GenericFunc: nil,
	// 	})
	//
	// if err != nil {
	// 	return nil, err
	// }

	err = c.Watch(
		source.Kind(mgr.GetCache(), &v1.Secret{}),
		handler.Funcs{
			CreateFunc: func(ctx context.Context, e event.CreateEvent, limitingInterface workqueue.RateLimitingInterface) {
				reconcileObj := e.Object
				if reconcileObj.GetName() == configSecretName {
					log.Info("SECRETS: Create Event. SECRET NAME:" + reconcileObj.GetName())

					selectedK8sNodes, err := GetKubernetesNodes(ctx, cl, reconcileObj)
					if err != nil {
						log.Error(err, "Failed get kubernetes nodes by labels")
						return
					}
					if len(selectedK8sNodes.Items) == 0 {
						log.Error(nil, "No Kubernetes nodes selected for LINSTOR. Check nodeSelector settings")
						return
					}

					for _, node := range selectedK8sNodes.Items {
						fmt.Printf("Node: %s\n", node.Name)
					}

					err = reconcileLinstorNodes(ctx, lc, selectedK8sNodes)
					if err != nil {
						log.Error(err, "Failed reconcile LINSTOR nodes")
						return
					}
				}

			},
			UpdateFunc: func(ctx context.Context, u event.UpdateEvent, limitingInterface workqueue.RateLimitingInterface) {
				reconcileObj := u.ObjectNew
				if reconcileObj.GetName() == configSecretName {
					log.Info("SECRETS: Update Event. SECRET NAME:" + reconcileObj.GetName())

					selectedK8sNodes, err := GetKubernetesNodes(ctx, cl, reconcileObj)
					if err != nil {
						log.Error(err, "Failed get kubernetes nodes by labels")
						return
					}
					if len(selectedK8sNodes.Items) == 0 {
						log.Error(nil, "No Kubernetes nodes selected for LINSTOR. Check nodeSelector settings")
						return
					}

					for _, node := range selectedK8sNodes.Items {
						fmt.Printf("Node: %s\n", node.Name)
					}

					err = reconcileLinstorNodes(ctx, lc, selectedK8sNodes)
					if err != nil {
						log.Error(err, "Failed reconcile LINSTOR nodes")
						return
					}

				}
			},
			DeleteFunc: func(ctx context.Context, e event.DeleteEvent, limitingInterface workqueue.RateLimitingInterface) {
				log.Error(nil, "Recieved DELETE Event for SECRET:"+e.Object.GetName()+". This secret contains configuration for this controller. Please recreate it")
				// TODO: return or die?

			},
			GenericFunc: nil,
		})

	return c, err

}

func reconcileLinstorNodes(ctx context.Context, lc *lclient.Client, selectedK8sNodes v1.NodeList) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, reachableTimeout)
	defer cancel()
	linstorNodes, err := lc.Nodes.GetAll(timeoutCtx, &lclient.ListOpts{})
	if err != nil {
		return err
	}

	// Create new Linstor node if it doesn't exist

	for _, selectedK8sNode := range selectedK8sNodes.Items {
		findMatch := false
		for _, linstorNode := range linstorNodes {
			if selectedK8sNode.Name == linstorNode.Name {
				findMatch = true
				break
			}
		}

		if findMatch {
			continue
		}

		fmt.Printf("Create LINSTOR node: %s\n", selectedK8sNode.Name)
		newLinstorNode := lclient.Node{
			Name: selectedK8sNode.Name,
			Type: LinstorSatelliteType,
			NetInterfaces: []lclient.NetInterface{
				{
					Name:                    "default",
					Address:                 net.ParseIP(selectedK8sNode.Status.Addresses[0].Address),
					IsActive:                true,
					SatellitePort:           LinstorNodePort,
					SatelliteEncryptionType: LinstorEncryptionType,
				},
			},
			Props: map[string]string{
				"Aux/registered-by": LinstorNodeControllerName,
			},
		}

		err := lc.Nodes.Create(ctx, newLinstorNode)
		if err != nil {
			return fmt.Errorf("unable to create node %s: %w", newLinstorNode.Name, err)
		}

	}

	// Drain and delete Linstor node if it doesn't present in selectedK8sNodes
	for _, linstorNode := range linstorNodes {
		findMatch := false
		fmt.Printf("Find LINSTOR node: %s\n", linstorNode.Name)

		for _, selectedK8sNode := range selectedK8sNodes.Items {
			if linstorNode.Name == selectedK8sNode.Name {
				findMatch = true
				break
			}
		}
		if findMatch {
			continue
		}

		// drain and delete node
		fmt.Printf("Drain and delete node: %s\n", linstorNode.Name) // TODO: implement drain logic

	}

	return err
}

func GetKubernetesNodes(ctx context.Context, cl client.Client, obj client.Object) (v1.NodeList, error) {
	selectedK8sNodes := v1.NodeList{}
	secret, ok := obj.(*v1.Secret)
	if !ok {
		return selectedK8sNodes, fmt.Errorf("err in type conversion from object to v1.Secret")
	}
	nodesLabels, err := getLabelsFromConfig(*secret)
	if err != nil {
		return selectedK8sNodes, err
	}

	err = cl.List(ctx, &selectedK8sNodes, client.MatchingLabels(nodesLabels))
	return selectedK8sNodes, err
}

func getLabelsFromConfig(secret v1.Secret) (map[string]string, error) {
	var secretConfig sdsapi.SdsDRBDOperatorConfig
	err := yaml.Unmarshal(secret.Data["config"], &secretConfig)
	if err != nil {
		return nil, err
	}
	labels := secretConfig.NodeSelector
	return labels, err
}
