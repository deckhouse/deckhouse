/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry_controller

import (
	"context"
	"embeded-registry-manager/internal/utils/k8s"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *RegistryReconciler) handleRegistryCaPKI(ctx context.Context, req ctrl.Request, secret *corev1.Secret) error {
	logger := ctrl.LoggerFrom(ctx)
	err := r.Get(ctx, req.NamespacedName, secret)

	// #TODO Check if error not 404
	if apierrors.IsNotFound(err) {
		logger.Info("Registry PKI was deleted", "Secret Name", req.NamespacedName.Name)

		// Recreate the registry PKI secret with existing CA data if it exists
		if r.embeddedRegistry.caPKI.Cert != nil && r.embeddedRegistry.caPKI.Key != nil &&
			r.embeddedRegistry.authTokenPKI.Cert != nil && r.embeddedRegistry.authTokenPKI.Key != nil {
			err := k8s.CreateRegistryCaPKISecret(ctx, r.Client,
				r.embeddedRegistry.caPKI.Cert, r.embeddedRegistry.caPKI.Key,
				r.embeddedRegistry.authTokenPKI.Cert, r.embeddedRegistry.authTokenPKI.Key,
			)
			if err != nil {
				return err
			} else {
				logger.Info("Recreated registry PKI secret with existing CA data")
				return nil
			}
		}

		_, caCertPEM, caKeyPEM, authTokenCertPEM, authTokenKeyPEM, err := k8s.EnsureCASecret(ctx, r.Client)
		if err != nil {
			return err
		}

		// Fill the embedded registry struct with the CA PKI and Auth Token PKI
		r.embeddedRegistry.caPKI = k8s.Certificate{
			Cert: caCertPEM,
			Key:  caKeyPEM,
		}
		r.embeddedRegistry.authTokenPKI = k8s.Certificate{
			Cert: authTokenCertPEM,
			Key:  authTokenKeyPEM,
		}
		logger.Info("New Registry registry-pki generated")
	}

	// If the secret exists, check if the CA certificate has changed TODO Check another fields
	if string(secret.Data[k8s.RegistryCAKey]) == string(r.embeddedRegistry.caPKI.Key) {
		logger.Info("Registry PKI not changed")
		return nil
	}
	logger.Info("Registry PKI changed")
	r.embeddedRegistry.caPKI = k8s.Certificate{
		Cert: secret.Data[k8s.RegistryCACert],
		Key:  secret.Data[k8s.RegistryCAKey],
	}
	r.embeddedRegistry.authTokenPKI = k8s.Certificate{
		Cert: secret.Data[k8s.AuthTokenCert],
		Key:  secret.Data[k8s.AuthTokenKey],
	}

	// Clear the master nodes slice
	for nodeName := range r.embeddedRegistry.masterNodes {
		delete(r.embeddedRegistry.masterNodes, nodeName)
	}

	// Delete all PKI secrets
	deletedSecrets, err := k8s.DeleteAllRegistryNodeSecrets(ctx, r.Client)
	if err != nil {
		return err
	}
	for _, deletedSecret := range deletedSecrets {
		logger.Info("Registry node PKI deleted secret due to CA certificate change", "secret", deletedSecret)
	}
	return nil
}
