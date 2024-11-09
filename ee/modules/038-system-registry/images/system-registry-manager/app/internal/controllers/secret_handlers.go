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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *RegistryReconciler) handleNodePKI(ctx context.Context, req ctrl.Request, nodeName string) (k8s.MasterNode, error) {
	logger := ctrl.LoggerFrom(ctx)
	secret := &corev1.Secret{}
	err := r.Get(ctx, req.NamespacedName, secret)

	// Recreate the node PKI secret if it was deleted
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Node PKI secret was deleted", "Secret Name", req.NamespacedName.Name)
			return r.recreateNodePKISecret(ctx, nodeName)
		} else {
			return k8s.MasterNode{}, err
		}
	}

	// #TODO
	return r.checkAndUpdateNodePKISecret(ctx, secret, nodeName)
}

func (r *RegistryReconciler) handleRegistryCaPKI(ctx context.Context, req ctrl.Request) error {
	logger := ctrl.LoggerFrom(ctx)
	secret := &corev1.Secret{}
	err := r.Get(ctx, req.NamespacedName, secret)

	if err != nil {
		// Check if the secret was deleted
		if apierrors.IsNotFound(err) {
			logger.Info("Registry PKI was deleted", "Secret Name", req.NamespacedName.Name)

			// Recreate the registry PKI secret with existing CA data if it exists
			if r.embeddedRegistry.caPKI.Cert != nil && r.embeddedRegistry.caPKI.Key != nil &&
				r.embeddedRegistry.authTokenPKI.Cert != nil && r.embeddedRegistry.authTokenPKI.Key != nil {
				err := k8s.CreateRegistryCaPKISecret(ctx, r.KubeClient,
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

			_, caCertPEM, caKeyPEM, authTokenCertPEM, authTokenKeyPEM, err := k8s.EnsureCASecret(ctx, r.KubeClient)
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
		// If the error is not NotFound, return it
		return err

	}

	// If the secret exists, check if the CA certificate has changed
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
	r.embeddedRegistry.masterNodes = nil

	// Delete all PKI secrets
	deletedSecrets, err := k8s.DeleteAllRegistryNodeSecrets(ctx, r.KubeClient)
	if err != nil {
		return err
	}
	for _, deletedSecret := range deletedSecrets {
		logger.Info("Registry node PKI deleted secret due to CA certificate change", "secret", deletedSecret)
	}
	return nil
}

func (r *RegistryReconciler) handleRegistryUser(ctx context.Context, req ctrl.Request, secretName string, user *k8s.RegistryUser) (bool, error) {
	logger := ctrl.LoggerFrom(ctx)

	secret := &corev1.Secret{}
	err := r.Get(ctx, req.NamespacedName, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Recreate the registry user secret with existing data if user is not empty
			if user.UserName != "" && user.Password != "" && user.HashedPassword != "" {
				_, err := k8s.CreateRegistryUserSecret(ctx, r.KubeClient, user.UserName, user.Password, user.HashedPassword)
				if err != nil {
					return false, err
				}
				logger.Info("Recreated registry user secret with existing data", "secretName", secretName)
				return false, nil
			}

			// Create the registry user secret with new credentials if user struct is empty
			user, err = k8s.CreateRegistryUser(ctx, r.KubeClient, secretName)
			if err != nil {
				return false, fmt.Errorf("failed to create new registry user secret: %w", err)
			}
			logger.Info("Created registry user secret with new credentials", "secretName", secretName)
			return true, nil
		}
		return false, err // Return the error if other error occurred
	}

	// Check if the registry user secret has changed
	if string(secret.Data["name"]) == user.UserName &&
		string(secret.Data["password"]) == user.Password &&
		string(secret.Data["passwordHash"]) == user.HashedPassword {
		logger.Info("Registry user password not changed", "Secret Name", req.NamespacedName.Name)
		return false, nil
	}

	// Update the user struct if the secret has changed
	user.UserName = string(secret.Data["name"])
	user.Password = string(secret.Data["password"])
	user.HashedPassword = string(secret.Data["passwordHash"])
	logger.Info("Registry user password changed", "Secret Name", req.NamespacedName.Name)
	return true, nil
}
