/*
Copyright 2026 Flant JSC

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
	"log/slog"

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
	ErrCacheNotSynced  = errors.New("password cache not synced")

	userOperationGVR = schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1",
		Resource: "useroperations",
	}
)

type Client interface {
	IsLocalUser(ctx context.Context, username string) (bool, error)
	CreatePasswordResetOperation(ctx context.Context, username, newPasswordHash string) (string, error)
	Start(ctx context.Context) error
	Stop()
}

type K8sClient struct {
	dynamicClient dynamic.Interface
	passwordCache *PasswordCache
	logger        *slog.Logger
}

func NewClient(logger *slog.Logger) (*K8sClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	passwordCache := NewPasswordCache(dynamicClient, logger)

	return &K8sClient{
		dynamicClient: dynamicClient,
		passwordCache: passwordCache,
		logger:        logger,
	}, nil
}

func NewClientWithDynamic(dynamicClient dynamic.Interface, logger *slog.Logger) *K8sClient {
	return &K8sClient{
		dynamicClient: dynamicClient,
		passwordCache: NewPasswordCache(dynamicClient, logger),
		logger:        logger,
	}
}

// Start initializes the password cache informer and waits for sync.
func (c *K8sClient) Start(ctx context.Context) error {
	return c.passwordCache.Start(ctx)
}

// Stop stops the password cache informer.
func (c *K8sClient) Stop() {
	c.passwordCache.Stop()
}

// IsLocalUser checks if a user is a local Dex user using the cached Password CRDs.
// Uses informer-based cache for O(1) lookups instead of API calls.
func (c *K8sClient) IsLocalUser(_ context.Context, username string) (bool, error) {
	if !c.passwordCache.IsSynced() {
		return false, ErrCacheNotSynced
	}
	return c.passwordCache.IsLocalUser(username), nil
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
