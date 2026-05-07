/*
Copyright 2023 Flant JSC

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

package hooks

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"golang.org/x/crypto/bcrypt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

type UserOperation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              UserOperationSpec   `json:"spec"`
	Status            UserOperationStatus `json:"status"`
}

type UserOperationSpec struct {
	User          string                `json:"user,omitempty"`
	Target        *UserOperationTarget  `json:"target,omitempty"`
	Type          UserOperationSpecType `json:"type"`
	InitiatorType string                `json:"initiatorType"`

	ResetPassword *UserOperationResetPasswordSpec `json:"resetPassword,omitempty"`
	Lock          *UserOperationLockSpec          `json:"lock,omitempty"`
}

// UserOperationTarget identifies an external (non-local) user managed by an
// authentication provider such as LDAP or Atlassian Crowd. It is mutually
// exclusive with UserOperationSpec.User and is used by the Lock / Unlock
// operations against the OfflineSessions object that holds the failed-attempt
// counter and the lock state for the corresponding (connectorID, email) pair.
type UserOperationTarget struct {
	ConnectorID string `json:"connectorID"`
	Email       string `json:"email"`
}

type UserOperationResetPasswordSpec struct {
	NewPasswordHash string `json:"newPasswordHash"`
}

type UserOperationLockSpec struct {
	For metav1.Duration `json:"for"`
}

type UserOperationSpecType string

const (
	UserOperationTypeResetPass = UserOperationSpecType("ResetPassword")
	UserOperationTypeReset2FA  = UserOperationSpecType("Reset2FA")
	UserOperationTypeLock      = UserOperationSpecType("Lock")
	UserOperationTypeUnlock    = UserOperationSpecType("Unlock")
)

type UserOperationStatus struct {
	Phase       UserOperationStatusPhase `json:"phase"`
	Message     string                   `json:"message,omitempty"`
	CompletedAt *metav1.Time             `json:"completedAt"`
}

type UserOperationStatusPhase string

const (
	UserOperationStatusPhaseSucceeded = UserOperationStatusPhase("Succeeded")
	UserOperationStatusPhaseFailed    = UserOperationStatusPhase("Failed")
)

// OfflineSessionSnapshot is a minimal representation of Dex OfflineSessions object used by this hook.
// We intentionally keep it flexible: different Dex versions/storages may store user identity differently,
// and OfflineSessions may not have userID at all but contain refresh token references.
type OfflineSessionSnapshot struct {
	Name            string       `json:"name"`
	Namespace       string       `json:"namespace"`
	UserID          string       `json:"userID"`
	ConnID          string       `json:"connID,omitempty"`
	Email           string       `json:"email,omitempty"`
	LockedUntil     *metav1.Time `json:"lockedUntil,omitempty"`
	RefreshTokenIDs []string     `json:"refreshTokenIDs,omitempty"`
}

// RefreshTokenSnapshot is a minimal representation of Dex RefreshToken object used by this hook.
type RefreshTokenSnapshot struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	ClaimsUserID    string `json:"claimsUserID,omitempty"`
	ClaimsUsername  string `json:"claimsUsername,omitempty"`
	ClaimsPreferred string `json:"claimsPreferredUsername,omitempty"`
}

const userOperationRetentionPeriod = 24 * time.Hour

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/user-authn",
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "*/5 * * * *"},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                "useroperations",
			ApiVersion:          "deckhouse.io/v1",
			Kind:                "UserOperation",
			FilterFunc:          applyUserOperationFilter,
			ExecuteHookOnEvents: ptr.To(true),
		},
		{
			Name:                "passwords",
			ApiVersion:          "dex.coreos.com/v1",
			Kind:                "Password",
			FilterFunc:          applyPasswordFilter,
			ExecuteHookOnEvents: ptr.To(false),
		},
		{
			Name:                "offlinesessions",
			ApiVersion:          "dex.coreos.com/v1",
			Kind:                "OfflineSessions",
			FilterFunc:          applyOfflineSessionFilter,
			ExecuteHookOnEvents: ptr.To(false),
		},
		{
			Name:                "refreshtokens",
			ApiVersion:          "dex.coreos.com/v1",
			Kind:                "RefreshToken",
			FilterFunc:          applyRefreshTokenFilter,
			ExecuteHookOnEvents: ptr.To(false),
		},
	},
}, getUserOperations)

func applyOfflineSessionFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	snap := &OfflineSessionSnapshot{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	// Be tolerant to different json field names / nesting. We only need user identity for Reset2FA.
	if v, found, _ := unstructured.NestedString(obj.Object, "userID"); found {
		snap.UserID = v
	} else if v, found, _ := unstructured.NestedString(obj.Object, "userId"); found {
		snap.UserID = v
	} else if v, found, _ := unstructured.NestedString(obj.Object, "spec", "userID"); found {
		snap.UserID = v
	} else if v, found, _ := unstructured.NestedString(obj.Object, "spec", "userId"); found {
		snap.UserID = v
	}

	if v, found, _ := unstructured.NestedString(obj.Object, "connID"); found {
		snap.ConnID = v
	} else if v, found, _ := unstructured.NestedString(obj.Object, "connId"); found {
		snap.ConnID = v
	}
	if v, found, _ := unstructured.NestedString(obj.Object, "email"); found {
		snap.Email = v
	}
	if v, found, _ := unstructured.NestedString(obj.Object, "lockedUntil"); found && v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			snap.LockedUntil = &metav1.Time{Time: t}
		}
	}

	// Collect refresh token IDs referenced by OfflineSessions. They can be used to infer user identity.
	if refreshMap, found, _ := unstructured.NestedMap(obj.Object, "refresh"); found && len(refreshMap) > 0 {
		ids := make([]string, 0, len(refreshMap))
		for _, v := range refreshMap {
			m, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if id, ok := m["ID"].(string); ok && id != "" {
				ids = append(ids, id)
				continue
			}
			// Be tolerant to different key casing.
			if id, ok := m["id"].(string); ok && id != "" {
				ids = append(ids, id)
				continue
			}
		}
		snap.RefreshTokenIDs = ids
	}

	return snap, nil
}

func applyRefreshTokenFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	snap := &RefreshTokenSnapshot{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	if v, found, _ := unstructured.NestedString(obj.Object, "claims", "userID"); found {
		snap.ClaimsUserID = v
	} else if v, found, _ := unstructured.NestedString(obj.Object, "claims", "userId"); found {
		snap.ClaimsUserID = v
	}
	if v, found, _ := unstructured.NestedString(obj.Object, "claims", "username"); found {
		snap.ClaimsUsername = v
	}
	if v, found, _ := unstructured.NestedString(obj.Object, "claims", "preferredUsername"); found {
		snap.ClaimsPreferred = v
	} else if v, found, _ := unstructured.NestedString(obj.Object, "claims", "preferred_username"); found {
		snap.ClaimsPreferred = v
	}

	return snap, nil
}

func applyUserOperationFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var userOperation = &UserOperation{}
	err := sdk.FromUnstructured(obj, userOperation)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return userOperation, nil
}

func getUserOperations(_ context.Context, input *go_hook.HookInput) error {
	operationsToExecute := make([]UserOperation, 0)
	operationsToCleanUp := make([]UserOperation, 0)
	for userOperation, err := range sdkobjectpatch.SnapshotIter[UserOperation](input.Snapshots.Get("useroperations")) {
		if err != nil {
			return fmt.Errorf("cannot map userOperation: cannot iterate over 'useroperations' snapshot: %v", err)
		}

		if userOperation.Status.Phase == "" {
			operationsToExecute = append(operationsToExecute, userOperation)
			continue
		}

		if time.Since(userOperation.GetObjectMeta().GetCreationTimestamp().Time) >= userOperationRetentionPeriod {
			operationsToCleanUp = append(operationsToCleanUp, userOperation)
		}
	}

	input.Logger.Info("Operations to execute", "count", len(operationsToExecute))
	input.Logger.Info("Operations to clean up", "count", len(operationsToCleanUp))

	for _, operation := range operationsToExecute {
		input.Logger.Info("Executing UserOperation", "name", operation.Name, "type", operation.Spec.Type)
		err := executeUserOperation(input, operation)
		if err != nil {
			input.Logger.Error(fmt.Sprintf("Failed to execute UserOperation %s: %v", operation.Name, err))
			operation.Status.Phase = UserOperationStatusPhaseFailed
			operation.Status.Message = err.Error()
		} else {
			input.Logger.Info("UserOperation succeeded", "name", operation.Name)
			operation.Status.Phase = UserOperationStatusPhaseSucceeded
			operation.Status.Message = ""
		}
		operation.Status.CompletedAt = ptr.To(metav1.Now())

		input.PatchCollector.PatchWithMerge(
			map[string]any{"status": operation.Status},
			"deckhouse.io/v1", "UserOperation", operation.Namespace, operation.Name,
			object_patch.WithSubresource("status"),
		)
	}

	for _, operation := range operationsToCleanUp {
		input.Logger.Info("Deleting old UserOperation", "name", operation.Name)
		input.PatchCollector.Delete("deckhouse.io/v1", "UserOperation", operation.Namespace, operation.Name)
	}
	return nil
}

func executeUserOperation(input *go_hook.HookInput, operation UserOperation) error {
	switch operation.Spec.Type {
	case UserOperationTypeResetPass:
		return executeResetPassword(input, operation)
	case UserOperationTypeReset2FA:
		return executeReset2FA(input, operation)
	case UserOperationTypeLock:
		return executeLock(input, operation)
	case UserOperationTypeUnlock:
		return executeUnlock(input, operation)
	default:
		return fmt.Errorf("unsupported operation type: %s", operation.Spec.Type)
	}
}

func executeLock(input *go_hook.HookInput, operation UserOperation) error {
	if operation.Spec.Lock == nil {
		input.Logger.Error("Lock spec is nil", "userOperation", operation.Name)
		return errors.New("lock spec is nil")
	}

	// Non-local users (LDAP, Crowd, ...): lock state lives in OfflineSessions
	// indexed by (email, connID).
	if operation.Spec.Target != nil {
		return lockOfflineSession(input, operation, operation.Spec.Lock.For.Duration)
	}

	var userPassword *Password
	for password, err := range sdkobjectpatch.SnapshotIter[Password](input.Snapshots.Get("passwords")) {
		if err != nil {
			return fmt.Errorf("cannot iter over password: %v", err)
		}
		if password.Username == operation.Spec.User {
			userPassword = &password
			break
		}
	}
	if userPassword == nil {
		return fmt.Errorf("cannot find password for user: %v", operation.Spec.User)
	}

	input.Logger.Info("Locking user password", "user", userPassword.Username, "duration", operation.Spec.Lock.For.Duration)
	input.PatchCollector.PatchWithMutatingFunc(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var pass Password
		if err := sdk.FromUnstructured(obj, &pass); err != nil {
			input.Logger.Error("Failed to convert Password object", "error", err)
			return nil, err
		}
		pass.LockedUntil = ptr.To(time.Now().Add(operation.Spec.Lock.For.Duration))
		u, err := sdk.ToUnstructured(&pass)
		if err != nil {
			return nil, err
		}
		annotations := u.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		// We need this annotations to find out who has banned user on user CR render later.
		annotations[PasswordAnnotationLockedByAdministrator] = ""
		u.SetAnnotations(annotations)

		return u, nil
	}, "dex.coreos.com/v1", "Password", userPassword.Namespace, userPassword.Name)

	return nil
}

func executeUnlock(input *go_hook.HookInput, operation UserOperation) error {
	if operation.Spec.Target != nil {
		return unlockOfflineSession(input, operation)
	}

	var userPassword *Password
	for password, err := range sdkobjectpatch.SnapshotIter[Password](input.Snapshots.Get("passwords")) {
		if err != nil {
			return fmt.Errorf("cannot iter over password: %v", err)
		}
		if password.Username == operation.Spec.User {
			userPassword = &password
			break
		}
	}
	if userPassword == nil {
		return fmt.Errorf("cannot find password for user: %v", operation.Spec.User)
	}

	input.Logger.Info("Unlocking user password", "user", userPassword.Username)
	input.PatchCollector.PatchWithMutatingFunc(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var pass Password
		if err := sdk.FromUnstructured(obj, &pass); err != nil {
			input.Logger.Error("Failed to convert Password object", "error", err)
			return nil, err
		}
		pass.LockedUntil = nil
		u, err := sdk.ToUnstructured(&pass)
		if err != nil {
			return nil, err
		}

		annotations := u.GetAnnotations()
		if annotations != nil {
			delete(annotations, PasswordAnnotationLockedByAdministrator)
			u.SetAnnotations(annotations)
		}

		return u, nil
	}, "dex.coreos.com/v1", "Password", userPassword.Namespace, userPassword.Name)

	return nil
}

func executeResetPassword(input *go_hook.HookInput, operation UserOperation) error {
	if operation.Spec.ResetPassword == nil {
		return errors.New("resetPassword spec is nil")
	}

	// Password.hash in Dex Password CR is base64-encoded bcrypt hash.
	// UserOperation.resetPassword.newPasswordHash must be a *raw* bcrypt hash, otherwise we risk
	// double-encoding and breaking logins.
	rawHash := operation.Spec.ResetPassword.NewPasswordHash
	if !strings.HasPrefix(rawHash, "$2") {
		return fmt.Errorf("resetPassword.newPasswordHash must be a raw bcrypt hash (starting with $2*), got: %q", rawHash)
	}
	if _, err := bcrypt.Cost([]byte(rawHash)); err != nil {
		return fmt.Errorf("resetPassword.newPasswordHash must be a valid bcrypt hash: %v", err)
	}

	var userPassword *Password
	for password, err := range sdkobjectpatch.SnapshotIter[Password](input.Snapshots.Get("passwords")) {
		if err != nil {
			return fmt.Errorf("cannot iter over password: %v", err)
		}
		if password.Username == operation.Spec.User {
			userPassword = &password
			break
		}
	}
	if userPassword == nil {
		return fmt.Errorf("cannot find password for user: %v", operation.Spec.User)
	}

	input.Logger.Info("Resetting user password", "user", userPassword.Username)
	input.PatchCollector.PatchWithMutatingFunc(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var pass Password
		if err := sdk.FromUnstructured(obj, &pass); err != nil {
			input.Logger.Error("Failed to convert Password object", "error", err)
			return nil, err
		}
		pass.Hash = base64.StdEncoding.EncodeToString(
			[]byte(rawHash),
		)
		pass.RequireResetHashOnNextSuccLogin = true
		return sdk.ToUnstructured(&pass)
	}, "dex.coreos.com/v1", "Password", userPassword.Namespace, userPassword.Name)

	return nil
}

func executeReset2FA(input *go_hook.HookInput, operation UserOperation) error {
	refreshTokensByID := make(map[string]RefreshTokenSnapshot, len(input.Snapshots.Get("refreshtokens")))
	for rt, err := range sdkobjectpatch.SnapshotIter[RefreshTokenSnapshot](input.Snapshots.Get("refreshtokens")) {
		if err != nil {
			return fmt.Errorf("cannot iter over RefreshTokens: %v", err)
		}
		// metadata.name is the refresh token ID
		refreshTokensByID[rt.Name] = rt
	}

	var anyDeleted bool

	for sess, err := range sdkobjectpatch.SnapshotIter[OfflineSessionSnapshot](input.Snapshots.Get("offlinesessions")) {
		if err != nil {
			return fmt.Errorf("cannot iter over OfflineSessions: %v", err)
		}

		matchesUser := false
		if sess.UserID != "" {
			matchesUser = (sess.UserID == operation.Spec.User)
		} else if len(sess.RefreshTokenIDs) > 0 {
			for _, id := range sess.RefreshTokenIDs {
				rt, ok := refreshTokensByID[id]
				if !ok {
					continue
				}
				if rt.ClaimsUsername == operation.Spec.User || rt.ClaimsUserID == operation.Spec.User || rt.ClaimsPreferred == operation.Spec.User {
					matchesUser = true
					break
				}
			}
		}

		if !matchesUser {
			input.Logger.Debug("OfflineSessions does not match requested user", "offlinesession", sess.Name, "userID", sess.UserID, "requestedUser", operation.Spec.User, "refreshTokenIDs", sess.RefreshTokenIDs)
			continue
		}

		input.Logger.Info("Resetting user 2FA: deleting OfflineSessions", "user", operation.Spec.User, "offlinesession", sess.Name)
		input.PatchCollector.Delete("dex.coreos.com/v1", "OfflineSessions", sess.Namespace, sess.Name)
		anyDeleted = true
	}

	// Also delete refresh tokens for the user to invalidate offline_access sessions and ensure consistent 2FA reset.
	for rt, err := range sdkobjectpatch.SnapshotIter[RefreshTokenSnapshot](input.Snapshots.Get("refreshtokens")) {
		if err != nil {
			return fmt.Errorf("cannot iter over RefreshTokens: %v", err)
		}
		if rt.ClaimsUsername == operation.Spec.User || rt.ClaimsUserID == operation.Spec.User || rt.ClaimsPreferred == operation.Spec.User {
			input.Logger.Info("Resetting user 2FA: deleting RefreshToken", "user", operation.Spec.User, "refreshtoken", rt.Name)
			input.PatchCollector.Delete("dex.coreos.com/v1", "RefreshToken", rt.Namespace, rt.Name)
			anyDeleted = true
		}
	}

	if !anyDeleted {
		input.Logger.Info("Reset2FA: no 2FA objects found, nothing to delete", "user", operation.Spec.User)
		return nil
	}

	return nil
}

// findOfflineSessionByTarget locates the OfflineSessions object that matches the
// (connectorID, email) pair from operation.Spec.Target. Email comparison is
// case-insensitive: connectors normalise to lower case but admins may type the
// email in any case in the UI.
func findOfflineSessionByTarget(input *go_hook.HookInput, target *UserOperationTarget) (*OfflineSessionSnapshot, error) {
	if target == nil {
		return nil, errors.New("target is nil")
	}
	if target.Email == "" || target.ConnectorID == "" {
		return nil, errors.New("target.connectorID and target.email are required")
	}

	wantEmail := strings.ToLower(target.Email)
	for sess, err := range sdkobjectpatch.SnapshotIter[OfflineSessionSnapshot](input.Snapshots.Get("offlinesessions")) {
		if err != nil {
			return nil, fmt.Errorf("cannot iter over OfflineSessions: %v", err)
		}
		if sess.ConnID != target.ConnectorID {
			continue
		}
		if strings.ToLower(sess.Email) != wantEmail {
			continue
		}
		sessCopy := sess
		return &sessCopy, nil
	}
	return nil, fmt.Errorf("no OfflineSessions found for connector %q and email %q (the user has likely never logged in yet)", target.ConnectorID, target.Email)
}

// lockOfflineSession patches OfflineSessions for a non-local user, setting
// LockedUntil and the deckhouse.io/locked-by-administrator annotation that the
// UI uses to distinguish admin-initiated locks from automatic ones.
//
// We use an explicit JSON merge patch (PatchWithMerge) instead of
// PatchWithMutatingFunc: the mutating-func variant computes a merge patch from
// the diff of mutated vs. source object, and on top-level CR fields like
// `lockedUntil` it produced a body in which neither `lockedUntil` nor
// `incorrectPasswordLoginAttempts` actually reached the apiserver — only the
// annotation slot did. Sending the desired values explicitly is the only
// reliable way to set top-level fields on a CR with
// x-kubernetes-preserve-unknown-fields. This mirrors unlockOfflineSession.
func lockOfflineSession(input *go_hook.HookInput, operation UserOperation, lockFor time.Duration) error {
	sess, err := findOfflineSessionByTarget(input, operation.Spec.Target)
	if err != nil {
		return err
	}

	input.Logger.Info("Locking external user via OfflineSessions",
		"connector", operation.Spec.Target.ConnectorID,
		"email", operation.Spec.Target.Email,
		"offlinesession", sess.Name,
		"duration", lockFor,
	)

	until := time.Now().Add(lockFor).UTC().Format(time.RFC3339)
	patch := map[string]any{
		"lockedUntil":                    until,
		"incorrectPasswordLoginAttempts": int64(0),
		"metadata": map[string]any{
			"annotations": map[string]any{
				// "true" matches what the Console UI writes for direct PATCHes
				// and what hasAdminLockAnnotation in the frontend treats as the
				// admin-lock marker; presence of the key is what actually
				// matters, but a stable value keeps both paths uniform.
				PasswordAnnotationLockedByAdministrator: "true",
			},
		},
	}

	input.PatchCollector.PatchWithMerge(patch, "dex.coreos.com/v1", "OfflineSessions", sess.Namespace, sess.Name)

	return nil
}

// unlockOfflineSession clears LockedUntil and the locked-by-administrator
// annotation, allowing the user to authenticate again immediately.
//
// We use an explicit JSON merge patch with nulls because PatchWithMutatingFunc
// computes a merge patch from the diff of mutated vs. source object: a removed
// field there becomes "absent" rather than null, which JSON merge patch
// semantics interpret as "leave unchanged" instead of "delete". Sending null
// values explicitly is the only reliable way to delete fields and annotation
// keys via merge patch.
func unlockOfflineSession(input *go_hook.HookInput, operation UserOperation) error {
	sess, err := findOfflineSessionByTarget(input, operation.Spec.Target)
	if err != nil {
		return err
	}

	input.Logger.Info("Unlocking external user via OfflineSessions",
		"connector", operation.Spec.Target.ConnectorID,
		"email", operation.Spec.Target.Email,
		"offlinesession", sess.Name,
	)

	patch := map[string]any{
		"lockedUntil":                    nil,
		"incorrectPasswordLoginAttempts": int64(0),
		"metadata": map[string]any{
			"annotations": map[string]any{
				PasswordAnnotationLockedByAdministrator: nil,
			},
		},
	}

	input.PatchCollector.PatchWithMerge(patch, "dex.coreos.com/v1", "OfflineSessions", sess.Namespace, sess.Name)

	return nil
}
