/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package k8s

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sort"
)

type MasterNode struct {
	Name                    string
	Address                 string
	AuthCertificate         Certificate
	DistributionCertificate Certificate
}

const (
	RegistryNamespace      = "d8-system"
	RegistryMcName         = "system-registry"
	ModuleConfigApiVersion = "deckhouse.io/v1alpha1"
	ModuleConfigKind       = "ModuleConfig"
	labelNodeRoleKey       = "node-role.kubernetes.io/master"
	RegistrySvcName        = labelModuleValue
)

// GetMasterNodeByName returns the master node with the given name
func GetMasterNodeByName(ctx context.Context, kubeClient *kubernetes.Clientset, nodeName string) (MasterNode, error) {
	// Get the node by name
	node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return MasterNode{}, err
	}

	// Check if the node has the master role
	if _, ok := node.Labels[labelNodeRoleKey]; !ok {
		return MasterNode{}, fmt.Errorf("node %s is not a master node", nodeName)
	}

	// Get the internal IP of the node
	internalIP, err := getNodeInternalIP(node)
	if err != nil {
		return MasterNode{}, err
	}

	// Create the MasterNode object
	masterNode := MasterNode{
		Name:    node.Name,
		Address: internalIP,
	}
	return masterNode, nil
}

// GetMasterNodes returns the master nodes in the cluster
func GetMasterNodes(ctx context.Context, kubeClient *kubernetes.Clientset) ([]MasterNode, error) {
	var masterNodes []MasterNode

	// Get all nodes with the master role
	nodesList, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labelNodeRoleKey,
	})
	if err != nil {
		return nil, err
	}

	// Sort the nodes by creation timestamp
	sort.Slice(nodesList.Items, func(i, j int) bool {
		return nodesList.Items[i].CreationTimestamp.Time.Before(nodesList.Items[j].CreationTimestamp.Time)
	})

	for _, node := range nodesList.Items {
		internalIP, err := getNodeInternalIP(&node)
		if err != nil {
			return nil, err
		}

		masterNode := MasterNode{
			Name:    node.Name,
			Address: internalIP,
		}
		masterNodes = append(masterNodes, masterNode)
	}

	//masterNodes = masterNodes[:1] // #TODO for now, we will use only the first master node

	return masterNodes, nil
}

// getNodeInternalIP returns the internal IP of the node
func getNodeInternalIP(node *corev1.Node) (string, error) {
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			return address.Address, nil
		}
	}
	return "", fmt.Errorf("internal IP not found for node %s", node.Name)
}
