/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package handler

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"sync"
)

const (
	masterNodeLabel   = "node-role.kubernetes.io/master="
	controlPlaneLabel = "node-role.kubernetes.io/control-plane="
)

type MasterNodesResource struct {
	data                      map[string]corev1.Node
	masterNodeLabelSelector   labels.Selector
	controlPlaneLabelSelector labels.Selector
	mu                        sync.Mutex
}

func NewMasterNodesResource() (*MasterNodesResource, error) {
	masterNodeLabelSelector, err := labels.Parse(masterNodeLabel)
	if err != nil {
		return nil, err
	}
	controlPlaneLabelSelector, err := labels.Parse(controlPlaneLabel)
	if err != nil {
		return nil, err
	}
	return &MasterNodesResource{
		data:                      make(map[string]corev1.Node),
		masterNodeLabelSelector:   masterNodeLabelSelector,
		controlPlaneLabelSelector: controlPlaneLabelSelector,
	}, nil
}

func (r *MasterNodesResource) Filter(obj interface{}) bool {
	if node, ok := obj.(*corev1.Node); ok {
		return (r.masterNodeLabelSelector.Matches(labels.Set(node.Labels)) || r.controlPlaneLabelSelector.Matches(labels.Set(node.Labels)))
	}
	return false
}

func (r *MasterNodesResource) OnAdd(obj interface{}, isInInitialList bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if node, ok := obj.(*corev1.Node); ok {
		r.data[node.Name] = *node
	} else {
		log.Error("Pars node error")
	}
}
func (r *MasterNodesResource) OnUpdate(oldObj, newObj interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if node, ok := newObj.(*corev1.Node); ok {
		r.data[node.Name] = *node
	} else {
		log.Error("Pars node error")
	}
}
func (r *MasterNodesResource) OnDelete(obj interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if node, ok := obj.(*corev1.Node); ok {
		delete(r.data, node.Name)
	} else {
		log.Error("Pars node error")
	}
}

func (r *MasterNodesResource) GetGroupVersionResourse() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "nodes",
	}
}

func (r *MasterNodesResource) GetData() map[string]corev1.Node {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.data
}

func GetNodeListByLabelSelector(labelSelector labels.Selector, clientSet *kubernetes.Clientset, namespace string) (*corev1.NodeList, error) {
	nodes, err := clientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("error getting NodeList: %v", err)
	}
	return nodes, nil
}
