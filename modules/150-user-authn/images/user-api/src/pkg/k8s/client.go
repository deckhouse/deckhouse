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

// UserOperation types and values user-api creates. A ValidatingAdmissionPolicy refuses any other
// UserOperation from the user-api service account.
const (
	OperationTypeLogout                = "Logout"
	OperationTypeRequirePasswordChange = "RequirePasswordChange"

	initiatorTypeSelf = "self"

	PhaseSucceeded = "Succeeded"
	PhaseFailed    = "Failed"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrAmbiguousUser   = errors.New("several local users have this user ID")
	ErrOperationFailed = errors.New("failed to create user operation")
	ErrCacheNotSynced  = errors.New("password cache not synced")

	userOperationGVR = schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1",
		Resource: "useroperations",
	}
)

// UserOperation is a self-service deckhouse.io/v1 UserOperation: the user acts on their own account.
type UserOperation struct {
	// GenerateName is the prefix of the generated object name.
	GenerateName string
	Type         string
	// User names a local user, Target the account of an external provider. Exactly one is set.
	User   string
	Target *Target
}

// Target names the account of an external provider: the email it signs in with, narrowed down to
// one provider and one user ID.
type Target struct {
	Email       string
	ConnectorID string
	UserID      string
}

// OperationStatus is what the controller writes to the status of a UserOperation. Phase is empty
// until the operation is done.
type OperationStatus struct {
	Phase   string
	Message string
}

type Client interface {
	// LocalUsername returns the username of the local user with the given user ID.
	LocalUsername(ctx context.Context, userID string) (string, error)
	// CreateUserOperation creates op and returns the generated name.
	CreateUserOperation(ctx context.Context, op UserOperation) (string, error)
	UserOperationStatus(ctx context.Context, name string) (OperationStatus, error)
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

	return NewClientWithDynamic(dynamicClient, logger)
}

func NewClientWithDynamic(dynamicClient dynamic.Interface, logger *slog.Logger) (*K8sClient, error) {
	passwordCache, err := NewPasswordCache(dynamicClient, logger)
	if err != nil {
		return nil, err
	}

	return &K8sClient{
		dynamicClient: dynamicClient,
		passwordCache: passwordCache,
		logger:        logger,
	}, nil
}

// Start initializes the password cache informer and waits for sync.
func (c *K8sClient) Start(ctx context.Context) error {
	return c.passwordCache.Start(ctx)
}

// Stop stops the password cache informer.
func (c *K8sClient) Stop() {
	c.passwordCache.Stop()
}

// LocalUsername looks the user up in the informer-based cache of Password objects.
func (c *K8sClient) LocalUsername(_ context.Context, userID string) (string, error) {
	return c.passwordCache.Username(userID)
}

func (c *K8sClient) CreateUserOperation(ctx context.Context, op UserOperation) (string, error) {
	spec := map[string]any{
		"type":          op.Type,
		"initiatorType": initiatorTypeSelf,
	}
	if op.User != "" {
		spec["user"] = op.User
	}
	if op.Target != nil {
		target := map[string]any{"email": op.Target.Email}
		if op.Target.ConnectorID != "" {
			target["connectorID"] = op.Target.ConnectorID
		}
		if op.Target.UserID != "" {
			target["userID"] = op.Target.UserID
		}
		spec["target"] = target
	}

	userOp := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "deckhouse.io/v1",
			"kind":       "UserOperation",
			"metadata": map[string]any{
				"generateName": op.GenerateName,
			},
			"spec": spec,
		},
	}

	created, err := c.dynamicClient.Resource(userOperationGVR).Create(ctx, userOp, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrOperationFailed, err)
	}

	return created.GetName(), nil
}

func (c *K8sClient) UserOperationStatus(ctx context.Context, name string) (OperationStatus, error) {
	op, err := c.dynamicClient.Resource(userOperationGVR).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return OperationStatus{}, fmt.Errorf("get user operation %s: %w", name, err)
	}

	phase, _, err := unstructured.NestedString(op.Object, "status", "phase")
	if err != nil {
		return OperationStatus{}, fmt.Errorf("read the phase of user operation %s: %w", name, err)
	}
	message, _, err := unstructured.NestedString(op.Object, "status", "message")
	if err != nil {
		return OperationStatus{}, fmt.Errorf("read the message of user operation %s: %w", name, err)
	}

	return OperationStatus{Phase: phase, Message: message}, nil
}
