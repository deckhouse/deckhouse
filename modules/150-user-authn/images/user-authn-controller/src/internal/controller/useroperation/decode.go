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
)

const (
	lockedByAdministratorAnnot = "deckhouse.io/locked-by-administrator"
	initiatorAnnot             = "deckhouse.io/initiator"

	lockForever = "permanent"

	typeResetPassword specType = "ResetPassword"
	typeReset2FA      specType = "Reset2FA"
	typeLock          specType = "Lock"
	typeUnlock        specType = "Unlock"

	phaseSucceeded statusPhase = "Succeeded"
	phaseFailed    statusPhase = "Failed"

	retentionPeriod = 24 * time.Hour
)

// foreverTime is lockedUntil for lock.for == permanent.
var foreverTime = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

type specType string
type statusPhase string

type operation struct {
	Name              string
	Namespace         string
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
}

type operationTarget struct {
	ConnectorID string `json:"connectorID"`
	Email       string `json:"email"`
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
}

type sessionView struct {
	Name            string
	Namespace       string
	UserID          string
	ConnID          string
	Email           string
	RefreshTokenIDs []string
}

type refreshTokenView struct {
	Name            string
	Namespace       string
	ClaimsUserID    string
	ClaimsUsername  string
	ClaimsPreferred string
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
	return passwordView{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Username:  username,
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

	refreshMap, found, err := unstructured.NestedMap(obj.Object, "refresh")
	if err != nil {
		return sessionView{}, fmt.Errorf("offlinesessions %s refresh: %w", obj.GetName(), err)
	}
	if found && len(refreshMap) > 0 {
		ids := make([]string, 0, len(refreshMap))
		for _, v := range refreshMap {
			m, ok := v.(map[string]any)
			if !ok {
				continue
			}
			id, ok := m["ID"].(string)
			if !ok || id == "" {
				id, ok = m["id"].(string)
			}
			if ok && id != "" {
				ids = append(ids, id)
			}
		}
		snap.RefreshTokenIDs = ids
	}
	return snap, nil
}

func decodeRefreshToken(obj *unstructured.Unstructured) (refreshTokenView, error) {
	if obj == nil {
		return refreshTokenView{}, fmt.Errorf("refreshtoken object is nil")
	}
	return refreshTokenView{
		Name:            obj.GetName(),
		Namespace:       obj.GetNamespace(),
		ClaimsUserID:    firstNestedString(obj.Object, [][]string{{"claims", "userID"}, {"claims", "userId"}}),
		ClaimsUsername:  firstNestedString(obj.Object, [][]string{{"claims", "username"}}),
		ClaimsPreferred: firstNestedString(obj.Object, [][]string{{"claims", "preferredUsername"}, {"claims", "preferred_username"}}),
	}, nil
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
	}
	return fields
}
