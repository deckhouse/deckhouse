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
	"fmt"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

type expirePatch struct {
	ExpireAt string   `json:"expireAt,omitempty"`
	Groups   []string `json:"groups"`
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
	ExpireAt string `json:"expireAt,omitempty"`
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
	},
}, getDexUsers)

func getDexUsers(input *go_hook.HookInput) error {
	users := make([]DexUserInternalValues, 0, len(input.Snapshots["users"]))
	mapOfUsersToGroups := map[string]map[string]bool{}

	groupsSnap := input.Snapshots["groups"]
	for _, obj := range groupsSnap {
		group := obj.(*DexGroup)
		makeUserGroupsMap(groupsSnap, group.Spec.Name, []string{}, mapOfUsersToGroups)
	}

	for _, user := range input.Snapshots["users"] {
		dexUser, ok := user.(*DexUser)
		if !ok {
			return fmt.Errorf("cannot convert user to dex user")
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

		users = append(users, DexUserInternalValues{
			Name:        dexUser.Name,
			EncodedName: encoding.ToFnvLikeDex(strings.ToLower(dexUser.Spec.Email)),
			Spec:        dexUser.Spec,
			Status:      dexUser.Status,
			ExpireAt:    expireAt,
		})

		var patch map[string]interface{}
		if expireAt == "" {
			patch = map[string]interface{}{
				"status": expirePatch{
					Groups: groups,
				},
			}
		} else {
			patch = map[string]interface{}{
				"status": expirePatch{
					ExpireAt: expireAt,
					Groups:   groups,
				},
			}
		}

		input.LogEntry.Infof("Update groups in user status %s. Groups: %v", dexUser.Name, patch["status"].(expirePatch).Groups)
		input.PatchCollector.MergePatch(patch, "deckhouse.io/v1", "User", "", dexUser.Name, object_patch.WithSubresource("/status"))
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

func findGroup(groups []go_hook.FilterResult, groupName string) *DexGroup {
	for _, obj := range groups {
		group := obj.(*DexGroup)
		if group.Spec.Name == groupName {
			return group
		}
	}
	return nil
}

func makeUserGroupsMap(groups []go_hook.FilterResult, targetGroup string, accumulatedGroupList []string, mapOfUsersToGroups map[string]map[string]bool) {
	if len(groups) == 0 {
		return
	}
	group := findGroup(groups, targetGroup)
	if group == nil {
		return
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
		if member.Kind == "User" {
			if mapOfUsersToGroups[member.Name] == nil {
				mapOfUsersToGroups[member.Name] = map[string]bool{}
			}
			for _, g := range accumulatedGroupList {
				mapOfUsersToGroups[member.Name][g] = true
			}
		} else if member.Kind == "Group" {
			makeUserGroupsMap(groups, member.Name, accumulatedGroupList, mapOfUsersToGroups)
		}
	}
}
