/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package legacy_controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	k8s "embeded-registry-manager/internal/utils/k8s_legacy"
)

func (r *RegistryReconciler) handleRegistryUser(ctx context.Context, req ctrl.Request, secretName string, user *k8s.RegistryUser, secret *corev1.Secret) (bool, error) {
	logger := ctrl.LoggerFrom(ctx)

	err := r.Get(ctx, req.NamespacedName, secret)
	if apierrors.IsNotFound(err) {
		// Recreate the registry user secret with existing data if user is not empty
		if user.UserName != "" && user.Password != "" && user.HashedPassword != "" {
			_, err := k8s.CreateRegistryUserSecret(ctx, r.Client, user.UserName, user.Password, user.HashedPassword)
			if err != nil {
				return false, err
			}
			logger.Info("Recreated registry user secret with existing data", "secretName", secretName)
			return false, nil
		}

		// Create the registry user secret with new credentials if user struct is empty
		user, err = k8s.CreateRegistryUser(ctx, r.Client, secretName)
		if err != nil {
			return false, fmt.Errorf("failed to create new registry user secret: %w", err)
		}

		logger.Info("Created registry user secret with new credentials", "secretName", secretName)
		return true, nil
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
