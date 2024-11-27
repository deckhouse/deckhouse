/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package legacy_controller

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	k8s "embeded-registry-manager/internal/utils/k8s_legacy"
)

var ErrNodeNotFound = errors.New("node not found in masterNodes")

func (r *RegistryReconciler) handleNodePKI(ctx context.Context, req ctrl.Request, nodeName string, secret *corev1.Secret) error {
	logger := ctrl.LoggerFrom(ctx)

	secret.Name = req.NamespacedName.Name
	secret.Namespace = req.NamespacedName.Namespace

	node, err := r.ensureNodePKISecret(ctx, secret, nodeName)
	if errors.Is(err, ErrNodeNotFound) {
		// Node has been removed from the cluster
		logger.Info("Node has been removed from the cluster", "Node Name", nodeName)

		// Delete secret
		err := r.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, secret)
		if err == nil {
			// Secret exists, delete it
			logger.Info("Deleting secret for removed node", "Secret Name", secret.Name)
			if err := r.Delete(ctx, secret); err != nil {
				logger.Error(err, "Failed to delete secret for removed node", "Secret Name", secret.Name)
				return err
			}
			// Mark secret as deleted
			r.deletedSecrets.Store(secret.Name, true)
		} else if !apierrors.IsNotFound(err) {
			// Error getting secret
			return err
		}

		// Delete static pod
		out, err := r.deleteNodeRegistry(ctx, nodeName)
		if err != nil {
			logger.Info("Error deleting static pod. Please delete the static manifest manually from the node", "Node Name", nodeName, "Response", string(out), "Error", err)
			return nil
		}

		logger.Info("Node has been removed from the cluster, static pod and secret have been removed", "Node Name", nodeName)
		return nil
	} else if err != nil {
		// return error not related to ErrNodeNotFound
		return err
	} else if node == nil {
		// Secret was deleted by controller and not recreated
		logger.Info("Secret was deleted by controller and not recreated", "Node Name", nodeName)
		return nil
	}

	return nil
	//return ctrl.Result{RequeueAfter: 60 * time.Second}, err
}

func isNodePKISecretUpToDate(secret *corev1.Secret, masterNode k8s.MasterNode) bool {

	return string(secret.Data[k8s.AuthCert]) == string(masterNode.AuthCertificate.Cert) &&
		string(secret.Data[k8s.AuthKey]) == string(masterNode.AuthCertificate.Key) &&
		string(secret.Data[k8s.DistributionCert]) == string(masterNode.DistributionCertificate.Cert) &&
		string(secret.Data[k8s.DistributionKey]) == string(masterNode.DistributionCertificate.Key)
}

func (r *RegistryReconciler) ensureNodePKISecret(ctx context.Context, secret *corev1.Secret, nodeName string) (*k8s.MasterNode, error) {
	logger := ctrl.LoggerFrom(ctx)

	masterNode, found := r.embeddedRegistry.masterNodes[nodeName]
	if !found {
		return nil, ErrNodeNotFound
	}

	//
	err := r.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, secret)
	if apierrors.IsNotFound(err) {
		logger.Info("Node PKI secret was deleted or not found", "Secret Name", secret.Name)

		//
		if _, exists := r.deletedSecrets.Load(secret.Name); exists {
			logger.Info("Secret was deleted by controller, not recreating", "Secret Name", secret.Name)
			//
			r.deletedSecrets.Delete(secret.Name)
			return nil, nil
		}

		//
		logger.Info("Recreating Node PKI Secret", "nodeName", nodeName)

		//
		dc, dk, ac, ak, err := k8s.CreateNodePKISecret(ctx, r.Client, masterNode, r.embeddedRegistry.caPKI.Cert, r.embeddedRegistry.caPKI.Key)
		if err != nil {
			return nil, err
		}

		//
		masterNode.DistributionCertificate = k8s.Certificate{Cert: dc, Key: dk}
		masterNode.AuthCertificate = k8s.Certificate{Cert: ac, Key: ak}
		r.embeddedRegistry.masterNodes[masterNode.Name] = masterNode

		logger.Info("Node PKI Secret recreated", "nodeName", masterNode.Name)
		return &masterNode, nil
	} else if err != nil {
		//
		return nil, err
	}

	//
	if isNodePKISecretUpToDate(secret, masterNode) {
		logger.Info("Node PKI Secret is up-to-date", "Secret Name", secret.Name)
		return &masterNode, nil
	}

	//
	masterNode.AuthCertificate = k8s.Certificate{
		Cert: secret.Data[k8s.AuthCert],
		Key:  secret.Data[k8s.AuthKey],
	}
	masterNode.DistributionCertificate = k8s.Certificate{
		Cert: secret.Data[k8s.DistributionCert],
		Key:  secret.Data[k8s.DistributionKey],
	}

	r.embeddedRegistry.masterNodes[masterNode.Name] = masterNode

	logger.Info("Node PKI Secret updated", "nodeName", masterNode.Name)
	return &masterNode, nil
}
