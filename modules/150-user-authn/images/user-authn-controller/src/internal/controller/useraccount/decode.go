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

package useraccount

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"user-authn-controller/internal/controller"
)

const (
	lockedByAdministratorAnnot = "deckhouse.io/locked-by-administrator"

	providerTypeLDAP  = "LDAP"
	providerTypeCrowd = "Crowd"
)

var (
	lockableProviderTypes = map[string]struct{}{
		providerTypeLDAP:  {},
		providerTypeCrowd: {},
	}
)

// passwordView is a Dex Password without hash/previousHashes.
type passwordView struct {
	Name        string
	Namespace   string
	Annotations map[string]string
	Email       string
	Username    string
	UserID      string
	Groups      []string
	Attempts    int64
	LockedUntil *time.Time
}

// sessionView is a Dex OfflineSessions without connectorData/refresh/totp.
type sessionView struct {
	Name        string
	Namespace   string
	Annotations map[string]string
	Email       string
	UserID      string
	ConnID      string
	Attempts    int64
	LockedUntil *time.Time
}

// userView is a deckhouse.io User without spec.password.
type userView struct {
	Name     string
	UID      types.UID
	Email    string
	UserID   string
	Groups   []string
	ExpireAt *time.Time
}

// providerView is a DexProvider without connector credentials.
type providerView struct {
	Name string
	Type string
}

type passwordFields struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Email             string   `json:"email,omitempty"`
	Username          string   `json:"username,omitempty"`
	UserID            string   `json:"userID,omitempty"`
	Groups            []string `json:"groups,omitempty"`
	LockedUntil       string   `json:"lockedUntil,omitempty"`
}

type sessionFields struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Email             string `json:"email,omitempty"`
	UserID            string `json:"userID,omitempty"`
	ConnID            string `json:"connID,omitempty"`
	LockedUntil       string `json:"lockedUntil,omitempty"`
}

type userFields struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              userSpecFields   `json:"spec,omitempty"`
	Status            userStatusFields `json:"status,omitempty"`
}

type userSpecFields struct {
	Email  string `json:"email,omitempty"`
	UserID string `json:"userID,omitempty"`
}

type userStatusFields struct {
	Groups   []string `json:"groups,omitempty"`
	ExpireAt string   `json:"expireAt,omitempty"`
}

type providerFields struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              providerSpecFields `json:"spec,omitempty"`
}

type providerSpecFields struct {
	Type string `json:"type,omitempty"`
}

func decodePassword(obj *unstructured.Unstructured) (passwordView, error) {
	var fields passwordFields
	if err := controller.DecodeInto(obj, &fields); err != nil {
		return passwordView{}, fmt.Errorf("decode password: %w", err)
	}
	attempts, err := controller.AsInt64(obj.Object["incorrectPasswordLoginAttempts"])
	if err != nil {
		return passwordView{}, fmt.Errorf("password %s incorrectPasswordLoginAttempts: %w", obj.GetName(), err)
	}
	until, err := controller.ParseRFC3339(fields.LockedUntil)
	if err != nil {
		return passwordView{}, fmt.Errorf("password %s lockedUntil: %w", obj.GetName(), err)
	}
	return passwordView{
		Name:        fields.Name,
		Namespace:   fields.Namespace,
		Annotations: fields.Annotations,
		Email:       fields.Email,
		Username:    fields.Username,
		UserID:      fields.UserID,
		Groups:      fields.Groups,
		Attempts:    attempts,
		LockedUntil: until,
	}, nil
}

func decodeSession(obj *unstructured.Unstructured) (sessionView, error) {
	var fields sessionFields
	if err := controller.DecodeInto(obj, &fields); err != nil {
		return sessionView{}, fmt.Errorf("decode offlinesessions: %w", err)
	}
	attempts, err := controller.AsInt64(obj.Object["incorrectPasswordLoginAttempts"])
	if err != nil {
		return sessionView{}, fmt.Errorf("offlinesessions %s incorrectPasswordLoginAttempts: %w", obj.GetName(), err)
	}
	until, err := controller.ParseRFC3339(fields.LockedUntil)
	if err != nil {
		return sessionView{}, fmt.Errorf("offlinesessions %s lockedUntil: %w", obj.GetName(), err)
	}
	return sessionView{
		Name:        fields.Name,
		Namespace:   fields.Namespace,
		Annotations: fields.Annotations,
		Email:       fields.Email,
		UserID:      fields.UserID,
		ConnID:      fields.ConnID,
		Attempts:    attempts,
		LockedUntil: until,
	}, nil
}

func decodeUser(obj *unstructured.Unstructured) (userView, error) {
	var fields userFields
	if err := controller.DecodeInto(obj, &fields); err != nil {
		return userView{}, fmt.Errorf("decode user: %w", err)
	}
	expireAt, err := controller.ParseRFC3339(fields.Status.ExpireAt)
	if err != nil {
		return userView{}, fmt.Errorf("user %s status.expireAt: %w", obj.GetName(), err)
	}
	return userView{
		Name:     fields.Name,
		UID:      fields.UID,
		Email:    fields.Spec.Email,
		UserID:   fields.Spec.UserID,
		Groups:   fields.Status.Groups,
		ExpireAt: expireAt,
	}, nil
}

func decodeProvider(obj *unstructured.Unstructured) (providerView, error) {
	var fields providerFields
	if err := controller.DecodeInto(obj, &fields); err != nil {
		return providerView{}, fmt.Errorf("decode dexprovider: %w", err)
	}
	return providerView{Name: fields.Name, Type: fields.Spec.Type}, nil
}

func hasAdminLock(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}
	_, ok := annotations[lockedByAdministratorAnnot]
	return ok
}

func isLockableProvider(providerType string) bool {
	_, ok := lockableProviderTypes[providerType]
	return ok
}
