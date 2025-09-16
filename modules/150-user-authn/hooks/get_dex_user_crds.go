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
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

type userStatusPatch struct {
	ExpireAt string      `json:"expireAt,omitempty"`
	Groups   []string    `json:"groups"`
	Lock     DexUserLock `json:"lock"`
}

type DexUserInternalValues struct {
	Name        string `json:"name"`
	EncodedName string `json:"encodedName"`

	Spec   DexUserSpec   `json:"spec"`
	Status DexUserStatus `json:"status,omitempty"`

	ExpireAt string `json:"-"`
}

type DexUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DexUserSpec   `json:"spec"`
	Status            DexUserStatus `json:"status,omitempty"`
}

type DexUserSpec struct {
	Email    string   `json:"email"`
	Password string   `json:"password"`
	UserID   string   `json:"userID,omitempty"`
	Groups   []string `json:"groups,omitempty"`
	TTL      string   `json:"ttl,omitempty"`
}

type DexUserStatus struct {
	ExpireAt string      `json:"expireAt,omitempty"`
	Lock     DexUserLock `json:"lock"`
}

type DexUserLockReason string

const (
	PasswordPolicyLockout = DexUserLockReason("PasswordPolicyLockout")
)

type DexUserLock struct {
	State   bool               `json:"state"`
	Reason  *DexUserLockReason `json:"reason,omitempty"`
	Message *string            `json:"message,omitempty"`
	Until   *string            `json:"until,omitempty"`
}

type DexGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DexGroupSpec   `json:"spec"`
	Status            DexGroupStatus `json:"status,omitempty"`
}

type DexGroupSpec struct {
	Name    string           `json:"name"`
	Members []DexGroupMember `json:"members" yaml:"members"`
}

type DexGroupMember struct {
	Kind string `json:"kind" yaml:"kind"`
	Name string `json:"name" yaml:"name"`
}

type DexGroupStatus struct {
	Errors []struct {
		Message   string `json:"message"`
		ObjectRef struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"objectRef"`
	} `json:"errors,omitempty"`
}

type Password struct {
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	LockedUntil *time.Time `json:"lockedUntil"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/user-authn",
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "*/5 * * * *"},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "users",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "User",
			FilterFunc: applyDexUserFilter,
		},
		{
			Name:       "groups",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "Group",
			FilterFunc: applyDexGroupFilter,
		},
		{
			Name:       "passwords",
			ApiVersion: "dex.coreos.com/v1",
			Kind:       "Password",
			FilterFunc: applyPasswordFilter,
		},
	},
}, getDexUsers)

func getDexUsers(_ context.Context, input *go_hook.HookInput) error {
	users := make([]DexUserInternalValues, 0, len(input.Snapshots.Get("users")))
	mapOfUsersToGroups := map[string]map[string]bool{}

	groupsSnap := input.Snapshots.Get("groups")
	for group, err := range sdkobjectpatch.SnapshotIter[DexGroup](groupsSnap) {
		if err != nil {
			return fmt.Errorf("cannot iterate over 'groups' snapshot: %v", err)
		}

		err = makeUserGroupsMap(groupsSnap, group.Spec.Name, []string{}, mapOfUsersToGroups, make(map[string]bool))
		if err != nil {
			return fmt.Errorf("error while make user groups map for group %s: %v", group.Spec.Name, err)
		}
	}

	for dexUser, err := range sdkobjectpatch.SnapshotIter[DexUser](input.Snapshots.Get("users")) {
		if err != nil {
			return fmt.Errorf("cannot convert user to dex user: cannot iterate over 'users' snapshot: %v", err)
		}

		userNameToPassword := make(map[string]Password)
		for password, err := range sdkobjectpatch.SnapshotIter[Password](input.Snapshots.Get("passwords")) {
			if err != nil {
				return fmt.Errorf("cannot convert user to password: cannot iterate over 'passwords' snapshot: %v", err)
			}

			userNameToPassword[password.Username] = password
		}

		var groups []string
		for g := range mapOfUsersToGroups[dexUser.Name] {
			groups = append(groups, g)
		}
		groups = set.New(groups...).Slice()

		dexUser.Spec.Groups = groups

		dexUser.Spec.UserID = dexUser.Name

		var expireAt string

		if dexUser.Status.ExpireAt == "" && dexUser.Spec.TTL != "" {
			parsedDuration, err := time.ParseDuration(dexUser.Spec.TTL)
			if err != nil {
				return fmt.Errorf("cannot parse expiration duration: %v", err)
			}

			expireAt = time.Now().Add(parsedDuration).Format(time.RFC3339)
			dexUser.Spec.TTL = ""
		} else {
			expireAt = dexUser.Status.ExpireAt
		}

		lock := DexUserLock{}
		password, ok := userNameToPassword[dexUser.Name]
		if ok && password.LockedUntil != nil && password.LockedUntil.After(time.Now()) {
			lock = DexUserLock{
				State:   true,
				Reason:  lo.ToPtr(PasswordPolicyLockout),
				Message: lo.ToPtr("Locked due to too many failed login attempts"),
				Until:   lo.ToPtr(password.LockedUntil.Format(time.RFC3339)),
			}
		}
		dexUser.Status.Lock = lock

		users = append(users, DexUserInternalValues{
			Name:        dexUser.Name,
			EncodedName: encoding.ToFnvLikeDex(strings.ToLower(dexUser.Spec.Email)),
			Spec:        dexUser.Spec,
			Status:      dexUser.Status,
			ExpireAt:    expireAt,
		})

		patch := userStatusPatch{
			Groups: groups,
			Lock:   lock,
		}
		if expireAt != "" {
			patch.ExpireAt = expireAt
		}
		patchMap := map[string]any{
			"status": patch,
		}

		input.Logger.Info("Sync user status", slog.Any("patch", patch))
		input.PatchCollector.PatchWithMerge(patchMap, "deckhouse.io/v1", "User", "", dexUser.Name, object_patch.WithSubresource("/status"))
	}

	input.Values.Set("userAuthn.internal.dexUsersCRDs", users)
	return nil
}

func applyDexGroupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var group = &DexGroup{}
	err := sdk.FromUnstructured(obj, group)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}
	return group, nil
}

func applyDexUserFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var user = &DexUser{}
	err := sdk.FromUnstructured(obj, user)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}
	return user, nil
}

func applyPasswordFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	password := &Password{}
	err := sdk.FromUnstructured(obj, password)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}
	return password, nil
}

func findGroup(groups []pkg.Snapshot, groupName string) (*DexGroup, error) {
	for group, err := range sdkobjectpatch.SnapshotIter[DexGroup](groups) {
		if err != nil {
			return nil, fmt.Errorf("cannot iterate over 'groups' snapshot: %v", err)
		}

		if group.Spec.Name == groupName {
			return &group, err
		}
	}
	return nil, nil
}

func makeUserGroupsMap(
	groups []pkg.Snapshot,
	targetGroup string,
	accumulatedGroupList []string,
	mapOfUsersToGroups map[string]map[string]bool,
	visited map[string]bool,
) error {
	if len(groups) == 0 {
		return nil
	}
	// If this group has already been visited, exit to prevent infinite recursion
	if visited[targetGroup] {
		return nil
	}
	visited[targetGroup] = true

	group, err := findGroup(groups, targetGroup)
	if err != nil {
		return fmt.Errorf("error while find group %s: %v", targetGroup, err)
	}
	if group == nil {
		return nil
	}

	skipAddGroup := false
	for _, g := range accumulatedGroupList {
		if g == targetGroup {
			skipAddGroup = true
		}
	}
	if !skipAddGroup {
		accumulatedGroupList = append(accumulatedGroupList, targetGroup)
	}
	for _, member := range group.Spec.Members {
		switch member.Kind {
		case "User":
			if mapOfUsersToGroups[member.Name] == nil {
				mapOfUsersToGroups[member.Name] = map[string]bool{}
			}
			for _, g := range accumulatedGroupList {
				mapOfUsersToGroups[member.Name][g] = true
			}
		case "Group":
			err := makeUserGroupsMap(groups, member.Name, accumulatedGroupList, mapOfUsersToGroups, visited)
			if err != nil {
				return fmt.Errorf("error while make user groups map for group %s: %v", member.Name, err)
			}
		}
	}
	return nil
}
