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

package user

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"user-authn-controller/internal/controller"
)

const (
	helmResourcePolicyAnnotation = "helm.sh/resource-policy"
	helmResourcePolicyKeep       = "keep"

	heritageLabel = "heritage"
	heritageValue = "deckhouse"
	moduleLabel   = "module"
	moduleValue   = "user-authn"
	appLabel      = "app"
	appValue      = "dex"

	lockedByAdministratorAnnot = "deckhouse.io/locked-by-administrator"

	lockReasonPasswordPolicy  = "PasswordPolicyLockout"
	lockReasonAdministrator   = "LockedByAdministrator"
	lockMessagePasswordPolicy = "Locked due to too many failed login attempts"
	lockMessageAdministrator  = "Locked by administrator"
)

type passwordView struct {
	Name                            string
	Namespace                       string
	Labels                          map[string]string
	Annotations                     map[string]string
	Email                           string
	Username                        string
	UserID                          string
	Hash                            string
	HashUpdatedAt                   string
	PreviousHashes                  []string
	Groups                          []string
	IncorrectPasswordLoginAttempts  int64
	RequireResetHashOnNextSuccLogin bool
	LockedUntil                     *time.Time
}

type userView struct {
	Name      string
	Email     string
	Password  string
	TTL       string
	ExpireAt  string
	Groups    []string
	Lock      userLock
	HasStatus bool
}

type userLock struct {
	State   bool   `json:"state"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
	Until   string `json:"until,omitempty"`
}

type groupView struct {
	SpecName string
	Members  []groupMember
}

type groupMember struct {
	Kind string
	Name string
}

type passwordFields struct {
	metav1.ObjectMeta               `json:"metadata,omitempty"`
	Email                           string   `json:"email,omitempty"`
	Username                        string   `json:"username,omitempty"`
	UserID                          string   `json:"userID,omitempty"`
	Hash                            string   `json:"hash,omitempty"`
	HashUpdatedAt                   string   `json:"hashUpdatedAt,omitempty"`
	PreviousHashes                  []string `json:"previousHashes,omitempty"`
	Groups                          []string `json:"groups,omitempty"`
	RequireResetHashOnNextSuccLogin bool     `json:"requireResetHashOnNextSuccLogin"`
	LockedUntil                     string   `json:"lockedUntil,omitempty"`
}

type userFields struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              userSpecFields   `json:"spec,omitempty"`
	Status            userStatusFields `json:"status,omitempty"`
}

type userSpecFields struct {
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
	TTL      string `json:"ttl,omitempty"`
}

type userStatusFields struct {
	ExpireAt string         `json:"expireAt,omitempty"`
	Groups   []string       `json:"groups,omitempty"`
	Lock     userLockFields `json:"lock,omitempty"`
}

type userLockFields struct {
	State   bool   `json:"state"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
	Until   string `json:"until,omitempty"`
}

type groupFields struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              groupSpecFields `json:"spec,omitempty"`
}

type groupSpecFields struct {
	Name    string              `json:"name,omitempty"`
	Members []groupMemberFields `json:"members,omitempty"`
}

type groupMemberFields struct {
	Kind string `json:"kind,omitempty"`
	Name string `json:"name,omitempty"`
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
		Name:                            fields.Name,
		Namespace:                       fields.Namespace,
		Labels:                          fields.Labels,
		Annotations:                     fields.Annotations,
		Email:                           fields.Email,
		Username:                        fields.Username,
		UserID:                          fields.UserID,
		Hash:                            fields.Hash,
		HashUpdatedAt:                   fields.HashUpdatedAt,
		PreviousHashes:                  fields.PreviousHashes,
		Groups:                          fields.Groups,
		IncorrectPasswordLoginAttempts:  attempts,
		RequireResetHashOnNextSuccLogin: fields.RequireResetHashOnNextSuccLogin,
		LockedUntil:                     until,
	}, nil
}

func decodeUser(obj *unstructured.Unstructured) (userView, error) {
	var fields userFields
	if err := controller.DecodeInto(obj, &fields); err != nil {
		return userView{}, fmt.Errorf("decode user: %w", err)
	}
	_, hasStatus, err := unstructured.NestedFieldNoCopy(obj.Object, "status")
	if err != nil {
		return userView{}, fmt.Errorf("user %s status: %w", obj.GetName(), err)
	}
	return userView{
		Name:      fields.Name,
		Email:     fields.Spec.Email,
		Password:  fields.Spec.Password,
		TTL:       fields.Spec.TTL,
		ExpireAt:  fields.Status.ExpireAt,
		Groups:    fields.Status.Groups,
		HasStatus: hasStatus,
		Lock: userLock{
			State:   fields.Status.Lock.State,
			Reason:  fields.Status.Lock.Reason,
			Message: fields.Status.Lock.Message,
			Until:   fields.Status.Lock.Until,
		},
	}, nil
}

func decodeGroup(obj *unstructured.Unstructured) (groupView, error) {
	var fields groupFields
	if err := controller.DecodeInto(obj, &fields); err != nil {
		return groupView{}, fmt.Errorf("decode group: %w", err)
	}
	members := make([]groupMember, 0, len(fields.Spec.Members))
	for _, m := range fields.Spec.Members {
		members = append(members, groupMember(m))
	}
	return groupView{SpecName: fields.Spec.Name, Members: members}, nil
}

func isModuleManagedPassword(p passwordView) bool {
	return p.Labels[heritageLabel] == heritageValue
}

func passwordObjectLabels() map[string]any {
	return map[string]any{
		heritageLabel: heritageValue,
		moduleLabel:   moduleValue,
		appLabel:      appValue,
	}
}

func toUnstructuredSlice(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}
