/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8s

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const (
	dexNamespace = "d8-user-authn"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrNotLocalUser    = errors.New("user is not a local user")
	ErrOperationFailed = errors.New("failed to create user operation")

	userOperationGVR = schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1",
		Resource: "useroperations",
	}

	passwordGVR = schema.GroupVersionResource{
		Group:    "dex.coreos.com",
		Version:  "v1",
		Resource: "passwords",
	}
)

type Client interface {
	IsLocalUser(ctx context.Context, username string) (bool, error)
	CreatePasswordResetOperation(ctx context.Context, username, newPasswordHash string) (string, error)
}

type K8sClient struct {
	dynamicClient dynamic.Interface
}

func NewClient() (*K8sClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &K8sClient{
		dynamicClient: dynamicClient,
	}, nil
}

func NewClientWithDynamic(dynamicClient dynamic.Interface) *K8sClient {
	return &K8sClient{
		dynamicClient: dynamicClient,
	}
}

func (c *K8sClient) IsLocalUser(ctx context.Context, username string) (bool, error) {
	passwords, err := c.dynamicClient.Resource(passwordGVR).Namespace(dexNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to list passwords: %w", err)
	}

	for _, pw := range passwords.Items {
		pwUsername, found, _ := unstructured.NestedString(pw.Object, "username")
		if found && pwUsername == username {
			return true, nil
		}
	}

	return false, nil
}

func (c *K8sClient) CreatePasswordResetOperation(ctx context.Context, username, newPasswordHash string) (string, error) {
	userOp := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deckhouse.io/v1",
			"kind":       "UserOperation",
			"metadata": map[string]interface{}{
				"generateName": "self-password-reset-",
			},
			"spec": map[string]interface{}{
				"user":          username,
				"type":          "ResetPassword",
				"initiatorType": "self",
				"resetPassword": map[string]interface{}{
					"newPasswordHash": newPasswordHash,
				},
			},
		},
	}

	created, err := c.dynamicClient.Resource(userOperationGVR).Create(ctx, userOp, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrOperationFailed, err)
	}

	return created.GetName(), nil
}
