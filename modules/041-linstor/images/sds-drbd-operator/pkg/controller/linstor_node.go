package controller

import (
	"context"
	"fmt"
	lclient "github.com/LINBIT/golinstor/client"
	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"net"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
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
	DRBDNodeSelectorKey       = "storage.deckhouse.io/sds-drbd-node"
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

			// fmt.Println("START EVENT ", request)

			if request.Name == configSecretName {
				log.Info("Start reconcile of LINSTOR nodes.")
				drbdNodeSelector := map[string]string{DRBDNodeSelectorKey: ""}
				err := reconcileLinstorNodes(ctx, cl, lc, log, request.Namespace, request.Name, drbdNodeSelector)
				if err != nil {
					log.Error(err, "Failed reconcile LINSTOR nodes")
					return reconcile.Result{
						RequeueAfter: time.Duration(interval) * time.Second,
					}, err
				}
			}

			return reconcile.Result{
				RequeueAfter: time.Duration(interval) * time.Second,
			}, nil

		}),
	})

	if err != nil {
		return nil, err
	}

	err = c.Watch(source.Kind(mgr.GetCache(), &v1.Secret{}), &handler.EnqueueRequestForObject{})

	return c, err

}

func reconcileLinstorNodes(ctx context.Context, cl client.Client, lc *lclient.Client, log logr.Logger, secretNamespace string, secretName string, drbdNodeSelector map[string]string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, reachableTimeout)
	defer cancel()

	log.Info("reconcileLinstorNodes: Get config from secret: " + secretNamespace + "/" + secretName)
	configSecret, err := GetKubernetesSecretByName(ctx, cl, secretName, secretNamespace)
	if err != nil {
		log.Error(err, "Failed get secret:"+secretName+"/"+secretNamespace)
		return err
	}
	configNodeSelector, err := GetNodeSelectorFromConfig(*configSecret)
	if err != nil {
		log.Error(err, "Failed get node selector from secret:"+secretName+"/"+secretNamespace)
		return err
	}
	selectedKubernetesNodes, err := GetKubernetesNodesBySelector(ctx, cl, configNodeSelector)
	if err != nil {
		log.Error(err, "Failed get nodes from Kubernetes by selector:"+fmt.Sprint(configNodeSelector))
		return err
	}

	log.Info("reconcileLinstorNodes: Get LINSTOR nodes")
	linstorNodes, err := lc.Nodes.GetAll(timeoutCtx, &lclient.ListOpts{})
	if err != nil {
		log.Error(err, "Failed get LINSTOR nodes")
		return err
	}
	log.Info("reconcileLinstorNodes: Start addDRBDNodes")
	err = addDRBDNodes(ctx, cl, lc, log, selectedKubernetesNodes, linstorNodes, drbdNodeSelector)

	// selectedDRBDNodes, err := GetKubernetesNodesBySelector(ctx, cl, map[string]string{DRBDNodeSelectorKey: ""})
	// if err != nil {
	// 	log.Error(err, "Failed get nodes from Kubernetes by selector:"+fmt.Sprint(map[string]string{DRBDNodeSelectorKey: ""}))
	// 	return err
	// }

	// allKubernetesNodes, err := GetAllKubernetesNodes(ctx, cl)
	// if err != nil {
	// 	log.Error(err, "Failed get all nodes from Kubernetes")
	// 	return err
	// }

	// drbdNodesToRemove := DiffNodeLists(selectedKubernetesNodes, allKubernetesNodes)

	// err = removeDRBDNodes(drbdNodesToRemove, linstorNodes)
	// drbdNodesToRemove := DiffNodeLists(selectedDRBDNodes, selectedKubernetesNodes)
	// for _, drbdNodeToAdd := range drbdNodesToAdd.Items {
	// 	fmt.Printf("New DRBD Node: %s\n", drbdNodeToAdd.Name)
	// }

	return nil
}

func addDRBDNodes(ctx context.Context, cl client.Client, lc *lclient.Client, log logr.Logger, selectedKubernetesNodes v1.NodeList, linstorNodes []lclient.Node, drbdNodeSelector map[string]string) error {
	log.Info("addDRBDNodes: Start")

	for _, selectedKubernetesNode := range selectedKubernetesNodes.Items {
		log.Info("addDRBDNodes: Process Kubernetes node: " + selectedKubernetesNode.Name)

		findMatch := false
		for _, linstorNode := range linstorNodes {
			if selectedKubernetesNode.Name == linstorNode.Name {
				findMatch = true
				break
			}
		}

		if !labels.Set(drbdNodeSelector).AsSelector().Matches(labels.Set(selectedKubernetesNode.Labels)) {
			log.Info("Kubernetes node: " + selectedKubernetesNode.Name + " doesn't have drbd label. Set it")

			originalNode := selectedKubernetesNode.DeepCopy()
			newNode := selectedKubernetesNode.DeepCopy()
			for labelKey, labelValue := range drbdNodeSelector {
				newNode.Labels[labelKey] = labelValue
			}

			err := cl.Patch(ctx, newNode, client.MergeFrom(originalNode))
			if err != nil {
				log.Error(err, "Unable set drbd labels to node %s. "+selectedKubernetesNode.Name)
			}
		}

		if findMatch {
			continue
		}

		fmt.Printf("Create LINSTOR node: %s\n", selectedKubernetesNode.Name)
		newLinstorNode := lclient.Node{
			Name: selectedKubernetesNode.Name,
			Type: LinstorSatelliteType,
			NetInterfaces: []lclient.NetInterface{
				{
					Name:                    "default",
					Address:                 net.ParseIP(selectedKubernetesNode.Status.Addresses[0].Address),
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
	return nil
}

func GetKubernetesSecretByName(ctx context.Context, cl client.Client, secretName string, secretNamespace string) (*v1.Secret, error) {
	secret := &v1.Secret{}
	err := cl.Get(ctx, client.ObjectKey{
		Name:      secretName,
		Namespace: secretNamespace,
	}, secret)
	return secret, err
}

func ReconcileLinstorNodes(ctx context.Context, lc *lclient.Client, selectedK8sNodes v1.NodeList) error {
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

func GetKubernetesNodesBySelector(ctx context.Context, cl client.Client, nodeSelector map[string]string) (v1.NodeList, error) {
	selectedK8sNodes := v1.NodeList{}
	err := cl.List(ctx, &selectedK8sNodes, client.MatchingLabels(nodeSelector))
	return selectedK8sNodes, err
}

func GetAllKubernetesNodes(ctx context.Context, cl client.Client) (v1.NodeList, error) {
	allKubernetesNodes := v1.NodeList{}
	err := cl.List(ctx, &allKubernetesNodes)
	return allKubernetesNodes, err
}

func GetNodeSelectorFromConfig(secret v1.Secret) (map[string]string, error) {
	var secretConfig sdsapi.SdsDRBDOperatorConfig
	err := yaml.Unmarshal(secret.Data["config"], &secretConfig)
	if err != nil {
		return nil, err
	}
	nodeSelector := secretConfig.NodeSelector
	return nodeSelector, err
}

func DiffNodeLists(leftList, rightList v1.NodeList) v1.NodeList {
	// diff := []string{}
	var diff v1.NodeList

	for _, leftNode := range leftList.Items {
		if !ContainsNode(rightList, leftNode) {
			// diff = append(diff, leftNode)
			diff.Items = append(diff.Items, leftNode)
		}
	}
	return diff
}

func ContainsNode(nodeList v1.NodeList, node v1.Node) bool {
	for _, item := range nodeList.Items {
		if item.Name == node.Name {
			return true
		}
	}
	return false

}
