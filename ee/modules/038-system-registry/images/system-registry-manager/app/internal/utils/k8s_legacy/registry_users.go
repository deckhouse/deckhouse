/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package k8s_legacy

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetRegistryUser returns the registry user by name
func GetRegistryUser(ctx context.Context, kc client.Client, userName string) (*RegistryUser, error) {
	// Get the secret by name
	var secret corev1.Secret
	err := kc.Get(ctx, types.NamespacedName{Name: userName, Namespace: RegistryNamespace}, &secret)
	if err != nil {
		return nil, err
	}

	return &RegistryUser{
		UserName:       string(secret.Data["name"]),
		Password:       string(secret.Data["password"]),
		HashedPassword: string(secret.Data["passwordHash"]),
	}, nil
}

// CreateRegistryUser creates a new registry user in the cluster
func CreateRegistryUser(ctx context.Context, kc client.Client, userName string) (*RegistryUser, error) {
	// Check if the secret already exists
	var secret corev1.Secret
	err := kc.Get(ctx, types.NamespacedName{Name: userName, Namespace: RegistryNamespace}, &secret)
	if !apierrors.IsNotFound(err) {
		return nil, err
	}

	if err == nil {
		return nil, fmt.Errorf("secret %s already exists", userName)
	}

	// Generate a new password
	password, hashedPassword, err := generateRegistryPassword()
	if err != nil {
		return nil, err
	}

	// Create the secret
	return CreateRegistryUserSecret(ctx, kc, userName, password, hashedPassword)
}
