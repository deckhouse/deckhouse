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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	userAccountGVR = schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: "useraccounts",
	}
)

// Kinds of a Device.
const (
	DeviceKindBrowser = "Browser"
	DeviceKindGrant   = "Grant"
)

// UserOperation is a self-service deckhouse.io/v1 UserOperation: the user acts on their own account.
type UserOperation struct {
	// GenerateName is the prefix of the generated object name.
	GenerateName string
	Type         string
	// User names a local user, Target the account of an external provider. Exactly one is set.
	User   string
	Target *Target
	// Logout narrows a Logout down to devices; nil signs the user out everywhere.
	Logout *Logout
}

// Logout is spec.logout of a UserOperation: at most one field is set. The controller ends a session
// or a grant only of the user the operation names.
type Logout struct {
	Session       string
	ExceptSession string
	Grant         string
	ExceptGrant   string
}

// Device is a device signed in to the account of a user: an entry of status.sessions of its
// UserAccount.
type Device struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	Clients    []string   `json:"clients,omitempty"`
	IPAddress  string     `json:"ipAddress,omitempty"`
	UserAgent  string     `json:"userAgent,omitempty"`
	SignedInAt *time.Time `json:"signedInAt,omitempty"`
	// KnownSince is when Dex first saw the browser of the device complete a login of the account, set
	// when earlier than SignedInAt: the device is as old as that.
	KnownSince   *time.Time `json:"knownSince,omitempty"`
	LastActivity *time.Time `json:"lastActivity,omitempty"`
}

// Devices are the devices of an account and whether the UserAccount lists only the most recently
// active of them.
type Devices struct {
	Items     []Device
	Truncated bool
	// UserID and ConnectorID are the pair the UserAccount belongs to, from its status.
	UserID      string
	ConnectorID string
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
	// AccountName returns the name of the UserAccount of the pair (userID, connectorID).
	AccountName(ctx context.Context, userID, connectorID string) (string, error)
	// Devices returns the devices of the UserAccount named name; none when there is no such object.
	Devices(ctx context.Context, name string) (Devices, error)
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

// AccountName names the UserAccount of a local user after its Password object, and that of an
// external provider after the pair.
func (c *K8sClient) AccountName(_ context.Context, userID, connectorID string) (string, error) {
	if connectorID == localConnectorID {
		return c.passwordCache.AccountName(userID)
	}
	return ExternalAccountName(connectorID, userID), nil
}

// Devices reads status.sessions of the UserAccount named name.
func (c *K8sClient) Devices(ctx context.Context, name string) (Devices, error) {
	account, err := c.dynamicClient.Resource(userAccountGVR).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return Devices{}, nil
	}
	if err != nil {
		return Devices{}, fmt.Errorf("get user account %s: %w", name, err)
	}

	var status struct {
		UserID            string   `json:"userID"`
		ConnectorID       string   `json:"connectorID"`
		Sessions          []Device `json:"sessions"`
		SessionsTruncated bool     `json:"sessionsTruncated"`
	}
	raw, found, err := unstructured.NestedFieldNoCopy(account.Object, "status")
	if err != nil || !found {
		return Devices{}, err
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return Devices{}, fmt.Errorf("encode the status of user account %s: %w", name, err)
	}
	if err := json.Unmarshal(data, &status); err != nil {
		return Devices{}, fmt.Errorf("decode the status of user account %s: %w", name, err)
	}
	return Devices{
		Items:       status.Sessions,
		Truncated:   status.SessionsTruncated,
		UserID:      status.UserID,
		ConnectorID: status.ConnectorID,
	}, nil
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
	if op.Logout != nil {
		logout := map[string]any{}
		for field, value := range map[string]string{
			"session":       op.Logout.Session,
			"exceptSession": op.Logout.ExceptSession,
			"grant":         op.Logout.Grant,
			"exceptGrant":   op.Logout.ExceptGrant,
		} {
			if value != "" {
				logout[field] = value
			}
		}
		spec["logout"] = logout
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
