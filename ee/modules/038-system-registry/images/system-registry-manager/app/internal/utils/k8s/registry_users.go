/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package k8s

import (
	"context"
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetRegistryUser returns the registry user by name
func GetRegistryUser(ctx context.Context, kubeClient *kubernetes.Clientset, userName string) (*RegistryUser, error) {
	// Get the secret by name
	secret, err := kubeClient.CoreV1().Secrets(RegistryNamespace).Get(ctx, userName, metav1.GetOptions{})
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
func CreateRegistryUser(ctx context.Context, kubeClient *kubernetes.Clientset, userName string) (*RegistryUser, error) {
	// Check if the secret already exists
	_, err := kubeClient.CoreV1().Secrets(RegistryNamespace).Get(ctx, userName, metav1.GetOptions{})
	if err == nil {
		return nil, fmt.Errorf("secret %s already exists", userName)
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}

	// Generate a new password
	password, hashedPassword, err := generateRegistryPassword()
	if err != nil {
		return nil, err
	}

	// Create the secret
	return CreateRegistryUserSecret(ctx, kubeClient, userName, password, hashedPassword)
}
