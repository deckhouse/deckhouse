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
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

type UserAction struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              UserActionSpec   `json:"spec"`
	Status            UserActionStatus `json:"status"`
}

type UserActionSpec struct {
	User          string             `json:"user"`
	Type          UserActionSpecType `json:"type"`
	InitiatorType string             `json:"initiatorType"`

	ResetPassword *UserActionResetPasswordSpec `json:"resetPassword,omitempty"`
	Lock          *UserActionLockSpec          `json:"lock,omitempty"`
}

type UserActionResetPasswordSpec struct {
	NewPasswordHash string `json:"newPasswordHash"`
}

type UserActionLockSpec struct {
	For metav1.Duration `json:"for"`
}

type UserActionSpecType string

const (
	UserActionTypeResetPass = UserActionSpecType("ResetPassword")
	UserActionTypeReset2FA  = UserActionSpecType("Reset2FA")
	UserActionTypeLock      = UserActionSpecType("Lock")
	UserActionTypeUnlock    = UserActionSpecType("Unlock")
)

type UserActionStatus struct {
	Phase       UserActionStatusPhase `json:"phase"`
	Message     string                `json:"message,omitempty"`
	CompletedAt *metav1.Time          `json:"completedAt"`
}

type UserActionStatusPhase string

const (
	UserActionStatusPhaseSucceeded = UserActionStatusPhase("Succeeded")
	UserActionStatusPhaseFailed    = UserActionStatusPhase("Failed")
)

type OfflineSession struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	UserID            string `json:"userID"`
	TOTPConfirmed     bool   `json:"totpConfirmed"`
}

const userActionRetentionPeriod = 24 * time.Hour

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/user-authn",
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "*/5 * * * *"},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                "useractions",
			ApiVersion:          "deckhouse.io/v1",
			Kind:                "UserAction",
			FilterFunc:          applyUserActionFilter,
			ExecuteHookOnEvents: lo.ToPtr(true),
		},
		{
			Name:                "passwords",
			ApiVersion:          "dex.coreos.com/v1",
			Kind:                "Password",
			FilterFunc:          applyPasswordFilter,
			ExecuteHookOnEvents: lo.ToPtr(false),
		},
		{
			Name:                "offlinesessions",
			ApiVersion:          "dex.coreos.com/v1",
			Kind:                "OfflineSessions",
			FilterFunc:          applyOfflineSessionFilter,
			ExecuteHookOnEvents: lo.ToPtr(false),
		},
	},
}, getUserActions)

func applyOfflineSessionFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var offlineSession = &OfflineSession{}
	err := sdk.FromUnstructured(obj, offlineSession)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return offlineSession, nil
}

func applyUserActionFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var userAction = &UserAction{}
	err := sdk.FromUnstructured(obj, userAction)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return userAction, nil
}

func getUserActions(_ context.Context, input *go_hook.HookInput) error {
	actionsToExecute := make([]UserAction, 0)
	actionsToCleanUp := make([]UserAction, 0)
	for userAction, err := range sdkobjectpatch.SnapshotIter[UserAction](input.Snapshots.Get("useractions")) {
		if err != nil {
			return fmt.Errorf("cannot map userAction: cannot iterate over 'useractions' snapshot: %v", err)
		}

		if userAction.Status.Phase == "" {
			actionsToExecute = append(actionsToExecute, userAction)
			continue
		}

		if time.Since(userAction.GetObjectMeta().GetCreationTimestamp().Time) >= userActionRetentionPeriod {
			actionsToCleanUp = append(actionsToCleanUp, userAction)
		}
	}

	input.Logger.Info("Actions to execute", "count", len(actionsToExecute))
	input.Logger.Info("Actions to clean up", "count", len(actionsToCleanUp))

	for _, action := range actionsToExecute {
		input.Logger.Info("Executing UserAction", "name", action.Name, "type", action.Spec.Type)
		err := executeUserAction(input, action)
		if err != nil {
			input.Logger.Error(fmt.Sprintf("Failed to execute UserAction %s: %v", action.Name, err))
			action.Status.Phase = UserActionStatusPhaseFailed
			action.Status.Message = err.Error()
		} else {
			input.Logger.Info("UserAction succeeded", "name", action.Name)
			action.Status.Phase = UserActionStatusPhaseSucceeded
			action.Status.Message = ""
		}
		action.Status.CompletedAt = lo.ToPtr(metav1.Now())

		input.PatchCollector.PatchWithMerge(
			map[string]any{"status": action.Status},
			"deckhouse.io/v1", "UserAction", action.Namespace, action.Name,
			object_patch.WithSubresource("status"),
		)
	}

	for _, action := range actionsToCleanUp {
		input.Logger.Info("Deleting old UserAction", "name", action.Name)
		input.PatchCollector.Delete("deckhouse.io/v1", "UserAction", action.Namespace, action.Name)
	}
	return nil
}

func executeUserAction(input *go_hook.HookInput, action UserAction) error {
	switch action.Spec.Type {
	case UserActionTypeResetPass:
		return executeResetPassword(input, action)
	case UserActionTypeReset2FA:
		return executeReset2FA(input, action)
	case UserActionTypeLock:
		return executeLock(input, action)
	case UserActionTypeUnlock:
		return executeUnlock(input, action)
	default:
		return fmt.Errorf("unsupported action type: %s", action.Spec.Type)
	}
}

func executeLock(input *go_hook.HookInput, action UserAction) error {
	if action.Spec.Lock == nil {
		input.Logger.Error("Lock spec is nil", "userAction", action.Name)
		return errors.New("lock spec is nil")
	}

	var userPassword *Password
	for password, err := range sdkobjectpatch.SnapshotIter[Password](input.Snapshots.Get("passwords")) {
		if err != nil {
			return fmt.Errorf("cannot iter over password: %v", err)
		}
		if password.Username == action.Spec.User {
			userPassword = &password
			break
		}
	}
	if userPassword == nil {
		return fmt.Errorf("cannot find password for user: %v", action.Spec.User)
	}

	input.Logger.Info("Locking user password", "user", userPassword.Username, "duration", action.Spec.Lock.For.Duration)
	input.PatchCollector.PatchWithMutatingFunc(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var pass Password
		if err := sdk.FromUnstructured(obj, &pass); err != nil {
			input.Logger.Error("Failed to convert Password object", "error", err)
			return nil, err
		}
		pass.LockedUntil = lo.ToPtr(time.Now().Add(action.Spec.Lock.For.Duration))
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

func executeUnlock(input *go_hook.HookInput, action UserAction) error {
	var userPassword *Password
	for password, err := range sdkobjectpatch.SnapshotIter[Password](input.Snapshots.Get("passwords")) {
		if err != nil {
			return fmt.Errorf("cannot iter over password: %v", err)
		}
		if password.Username == action.Spec.User {
			userPassword = &password
			break
		}
	}
	if userPassword == nil {
		return fmt.Errorf("cannot find password for user: %v", action.Spec.User)
	}

	input.Logger.Info("Unlocking user password", "user", userPassword.Username)
	input.PatchCollector.PatchWithMutatingFunc(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var pass Password
		if err := sdk.FromUnstructured(obj, &pass); err != nil {
			input.Logger.Error("Failed to convert Password object", "error", err)
			return nil, err
		}
		pass.LockedUntil = nil
		return sdk.ToUnstructured(&pass)
	}, "dex.coreos.com/v1", "Password", userPassword.Namespace, userPassword.Name)

	return nil
}

func executeResetPassword(input *go_hook.HookInput, action UserAction) error {
	if action.Spec.ResetPassword == nil {
		return errors.New("resetPassword spec is nil")
	}

	var userPassword *Password
	for password, err := range sdkobjectpatch.SnapshotIter[Password](input.Snapshots.Get("passwords")) {
		if err != nil {
			return fmt.Errorf("cannot iter over password: %v", err)
		}
		if password.Username == action.Spec.User {
			userPassword = &password
			break
		}
	}
	if userPassword == nil {
		return fmt.Errorf("cannot find password for user: %v", action.Spec.User)
	}

	input.Logger.Info("Resetting user password", "user", userPassword.Username)
	input.PatchCollector.PatchWithMutatingFunc(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var pass Password
		if err := sdk.FromUnstructured(obj, &pass); err != nil {
			input.Logger.Error("Failed to convert Password object", "error", err)
			return nil, err
		}
		pass.Hash = base64.StdEncoding.EncodeToString(
			[]byte(action.Spec.ResetPassword.NewPasswordHash),
		)
		pass.RequireResetHashOnNextSuccLogin = true
		return sdk.ToUnstructured(&pass)
	}, "dex.coreos.com/v1", "Password", userPassword.Namespace, userPassword.Name)

	return nil
}

func executeReset2FA(input *go_hook.HookInput, action UserAction) error {
	var someOfflineSessionDeleted bool
	for sess, err := range sdkobjectpatch.SnapshotIter[OfflineSession](input.Snapshots.Get("offlinesessions")) {
		if err != nil {
			return fmt.Errorf("cannot iter over OfflineSessions: %v", err)
		}
		if sess.UserID != action.Spec.User {
			input.Logger.Error("session.UserID != action.Spec.User", "sess.UserID", sess.UserID, "action.Spec.User", action.Spec.User)
			continue
		}

		input.Logger.Info("Resetting user 2FA", "user", sess.UserID)
		input.PatchCollector.Delete("dex.coreos.com/v1", "OfflineSession", sess.Namespace, sess.Name)
		someOfflineSessionDeleted = true
	}
	if !someOfflineSessionDeleted {
		return fmt.Errorf("cannot find user's 2FA objects: %v", action.Spec.User)
	}

	return nil
}
