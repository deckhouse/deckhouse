/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controllers

import (
	"context"
	"embeded-registry-manager/internal/utils/k8s"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *RegistryReconciler) recreateNodePKISecret(ctx context.Context, nodeName string) (k8s.MasterNode, error) {
	logger := ctrl.LoggerFrom(ctx)

	// Get the master node by name
	masterNode, err := k8s.GetMasterNodeByName(ctx, r.KubeClient, nodeName)
	if err != nil {
		return k8s.MasterNode{}, err
	}

	// Create the node PKI secret
	dc, dk, ac, ak, err := k8s.CreateNodePKISecret(ctx, r.KubeClient, masterNode, r.embeddedRegistry.caPKI.Cert, r.embeddedRegistry.caPKI.Key)
	if err != nil {
		return k8s.MasterNode{}, err
	}

	// Fill the master node struct with the certificates
	masterNode.DistributionCertificate = k8s.Certificate{Cert: dc, Key: dk}
	masterNode.AuthCertificate = k8s.Certificate{Cert: ac, Key: ak}

	// Add the node to the embedded registry struct
	r.embeddedRegistry.updateMasterNode(masterNode)

	logger.Info("Node secret recreated", "nodeName", masterNode.Name)
	return masterNode, nil
}

func (r *RegistryReconciler) checkAndUpdateNodePKISecret(ctx context.Context, secret *corev1.Secret, nodeName string) (k8s.MasterNode, error) {
	logger := ctrl.LoggerFrom(ctx)

	// Get the master node by name
	masterNode, found := r.embeddedRegistry.getMasterNodeFromEmbeddedRegistryStruct(nodeName)
	if !found {
		return k8s.MasterNode{}, fmt.Errorf("master node %s not found in embeddedRegistry", nodeName)
	}

	// Check if the node PKI secret has changed
	if isNodePKISecretUpToDate(secret, masterNode) {
		logger.Info("Registry Node PKI not changed", "Secret Name", secret.Name)
		return masterNode, nil
	}

	// If the secret has changed, update the master node struct
	masterNode.AuthCertificate = k8s.Certificate{
		Cert: secret.Data[k8s.AuthCert],
		Key:  secret.Data[k8s.AuthKey],
	}
	masterNode.DistributionCertificate = k8s.Certificate{
		Cert: secret.Data[k8s.DistributionCert],
		Key:  secret.Data[k8s.DistributionKey],
	}

	logger.Info("Registry Node PKI changed", "node name", masterNode.Name, "Secret Name", secret.Name)
	return masterNode, nil
}

func isNodePKISecretUpToDate(secret *corev1.Secret, masterNode k8s.MasterNode) bool {

	return string(secret.Data[k8s.AuthCert]) == string(masterNode.AuthCertificate.Cert) &&
		string(secret.Data[k8s.AuthKey]) == string(masterNode.AuthCertificate.Key) &&
		string(secret.Data[k8s.DistributionCert]) == string(masterNode.DistributionCertificate.Cert) &&
		string(secret.Data[k8s.DistributionKey]) == string(masterNode.DistributionCertificate.Key)
}
