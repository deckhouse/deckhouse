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

package useroperation

import (
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"user-authn-controller/internal/useridentity"
)

const (
	lockedByAdministratorAnnot = "deckhouse.io/locked-by-administrator"
	initiatorAnnot             = "deckhouse.io/initiator"

	lockForever = "permanent"

	typeResetPassword         specType = "ResetPassword"
	typeReset2FA              specType = "Reset2FA"
	typeLock                  specType = "Lock"
	typeUnlock                specType = "Unlock"
	typeLogout                specType = "Logout"
	typeRequirePasswordChange specType = "RequirePasswordChange"

	phaseSucceeded statusPhase = "Succeeded"
	phaseFailed    statusPhase = "Failed"

	// DefaultRetention is how long a UserOperation is kept after its creation, unless configured.
	DefaultRetention = 24 * time.Hour
)

// foreverTime is lockedUntil for lock.for == permanent.
var foreverTime = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

type specType string
type statusPhase string

type operation struct {
	Name              string
	Namespace         string
	UID               string // marks the Revocation objects the operation writes, so that a retry finds them
	CreationTimestamp metav1.Time
	Annotations       map[string]string
	Spec              operationSpec
	Status            operationStatus
	DecodeErr         error
}

type operationSpec struct {
	User          string           `json:"user,omitempty"`
	Target        *operationTarget `json:"target,omitempty"`
	Type          specType         `json:"type"`
	InitiatorType string           `json:"initiatorType"`
	ResetPassword *resetPassword   `json:"resetPassword,omitempty"`
	Lock          *lockSpec        `json:"lock,omitempty"`
	Logout        *logoutSpec      `json:"logout,omitempty"`
}

// operationTarget names an account by email; connectorID and userID narrow it down.
type operationTarget struct {
	ConnectorID string `json:"connectorID,omitempty"`
	Email       string `json:"email"`
	UserID      string `json:"userID,omitempty"`
}

// logoutSpec narrows a Logout down to one browser session, every session but one, one client, one
// grant (a device without a browser session), or every grant but one. Without any of them the
// Logout revokes everything of the pair.
type logoutSpec struct {
	Session       string `json:"session,omitempty"`
	ExceptSession string `json:"exceptSession,omitempty"`
	Grant         string `json:"grant,omitempty"`
	ExceptGrant   string `json:"exceptGrant,omitempty"`
}

type resetPassword struct {
	NewPasswordHash string `json:"newPasswordHash"`
}

type lockSpec struct {
	For string `json:"for"`
}

type operationStatus struct {
	Phase       statusPhase `json:"phase"`
	Message     string      `json:"message,omitempty"`
	CompletedAt string      `json:"completedAt"`
}

type passwordView struct {
	Name      string
	Namespace string
	Username  string
	UserID    string
	Email     string
	Groups    []string
}

type sessionView struct {
	Name      string
	Namespace string
	UserID    string
	ConnID    string
	Email     string
}

// identityView is a Dex UserIdentity as far as the lookup of spec.target needs it.
type identityView struct {
	UserID      string
	ConnectorID string
	Email       string
}

type failedError struct {
	msg string
}

func (e failedError) Error() string { return e.msg }

func failed(msg string) error { return failedError{msg: msg} }

func failedf(format string, args ...any) error {
	return failedError{msg: fmt.Sprintf(format, args...)}
}

func decodeOperation(obj *unstructured.Unstructured) operation {
	op := operation{
		Name:              obj.GetName(),
		Namespace:         obj.GetNamespace(),
		UID:               string(obj.GetUID()),
		CreationTimestamp: obj.GetCreationTimestamp(),
		Annotations:       obj.GetAnnotations(),
	}
	if phase, found, err := unstructured.NestedString(obj.Object, "status", "phase"); err == nil && found {
		op.Status.Phase = statusPhase(phase)
	}
	if msg, found, err := unstructured.NestedString(obj.Object, "status", "message"); err == nil && found {
		op.Status.Message = msg
	}

	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		op.DecodeErr = failedf("cannot decode UserOperation object: %s", err.Error())
		return op
	}
	if !found {
		op.DecodeErr = failed("cannot decode UserOperation object: spec is missing")
		return op
	}
	raw, err := json.Marshal(spec)
	if err != nil {
		op.DecodeErr = failedf("cannot decode UserOperation object: %s", err.Error())
		return op
	}
	if err := json.Unmarshal(raw, &op.Spec); err != nil {
		op.DecodeErr = failedf("cannot decode UserOperation object: %s", err.Error())
		return op
	}
	return op
}

func decodePassword(obj *unstructured.Unstructured) (passwordView, error) {
	if obj == nil {
		return passwordView{}, fmt.Errorf("password object is nil")
	}
	username, _, err := unstructured.NestedString(obj.Object, "username")
	if err != nil {
		return passwordView{}, fmt.Errorf("password %s username: %w", obj.GetName(), err)
	}
	groups, _, err := unstructured.NestedStringSlice(obj.Object, "groups")
	if err != nil {
		// groups only seeds the claims of a new UserIdentity; a null or malformed value is no
		// reason to fail a lock.
		groups = nil
	}
	return passwordView{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Username:  username,
		UserID:    firstNestedString(obj.Object, [][]string{{"userID"}}),
		Email:     firstNestedString(obj.Object, [][]string{{"email"}}),
		Groups:    groups,
	}, nil
}

func decodeSession(obj *unstructured.Unstructured) (sessionView, error) {
	if obj == nil {
		return sessionView{}, fmt.Errorf("offlinesessions object is nil")
	}
	snap := sessionView{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		UserID:    firstNestedString(obj.Object, [][]string{{"userID"}, {"userId"}, {"spec", "userID"}, {"spec", "userId"}}),
		ConnID:    firstNestedString(obj.Object, [][]string{{"connID"}, {"connId"}}),
		Email:     firstNestedString(obj.Object, [][]string{{"email"}}),
	}

	return snap, nil
}

func decodeIdentity(obj *unstructured.Unstructured) (identityView, error) {
	if obj == nil {
		return identityView{}, fmt.Errorf("useridentity object is nil")
	}
	id := useridentity.FromUnstructured(obj)
	claims, err := id.Claims()
	if err != nil {
		return identityView{}, err
	}
	return identityView{UserID: id.UserID(), ConnectorID: id.ConnectorID(), Email: claims.Email}, nil
}

func firstNestedString(obj map[string]any, paths [][]string) string {
	for _, path := range paths {
		v, found, err := unstructured.NestedString(obj, path...)
		if err != nil || !found || v == "" {
			continue
		}
		return v
	}
	return ""
}

func operationLogKV(op operation) []any {
	fields := []any{
		"operation", op.Name,
		"namespace", op.Namespace,
		"type", op.Spec.Type,
		"initiatorType", op.Spec.InitiatorType,
		"createdAt", op.CreationTimestamp.UTC().Format(time.RFC3339),
	}
	if initiator := op.Annotations[initiatorAnnot]; initiator != "" {
		fields = append(fields, "initiator", initiator)
	}
	if op.Spec.User != "" {
		fields = append(fields, "targetKind", "local", "targetUser", op.Spec.User)
	}
	if op.Spec.Target != nil {
		fields = append(fields,
			"targetKind", "external",
			"targetConnector", op.Spec.Target.ConnectorID,
			"targetEmail", op.Spec.Target.Email,
		)
		if op.Spec.Target.UserID != "" {
			fields = append(fields, "targetUserID", op.Spec.Target.UserID)
		}
	}
	if l := op.Spec.Logout; l != nil {
		fields = append(fields, "logoutSession", l.Session, "logoutExceptSession", l.ExceptSession,
			"logoutGrant", l.Grant, "logoutExceptGrant", l.ExceptGrant)
	}
	return fields
}
