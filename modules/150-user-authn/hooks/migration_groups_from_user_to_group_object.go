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
	"hash/fnv"
	"regexp"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

const (
	DexGroupKind       = "Group"
	DexGroupGroup      = "deckhouse.io"
	DexGroupVersion    = "v1alpha1"
	DexGroupResource   = "groups"
	DexGroupAPIVersion = DexGroupGroup + "/" + DexGroupVersion
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "users",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "User",
			FilterFunc:                   applyDexUserFilter,
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
		},
		{
			Name:       "migrated",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"user-authn-groups-migrated"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			ExecuteHookOnSynchronization: ptr.To(false),
			ExecuteHookOnEvents:          ptr.To(false),
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				return obj.GetName(), nil
			},
		},
	},
}, migrationGroups)

func migrationGroups(input *go_hook.HookInput) error {
	if len(input.Snapshots["migrated"]) > 0 {
		// We need this hook to run only once
		return nil
	}

	groupToUsersMap := make(map[string][]string)
	for _, obj := range input.Snapshots["users"] {
		user := obj.(*DexUser)
		for _, group := range user.Spec.Groups {
			groupToUsersMap[group] = append(groupToUsersMap[group], user.Name)
		}
	}

	for groupName, users := range groupToUsersMap {
		var members []DexGroupMember
		for _, userName := range users {
			members = append(members, DexGroupMember{Kind: "User", Name: userName})
		}
		dexGroup := &DexGroup{
			TypeMeta: metav1.TypeMeta{
				Kind:       DexGroupKind,
				APIVersion: DexGroupAPIVersion,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: metadataNameify(groupName),
			},
			Spec: DexGroupSpec{
				Name:    groupName,
				Members: members,
			},
		}

		input.Logger.Infof("Create Group %s with members %s", dexGroup.Spec.Name, dexGroup.Spec.Members)
		input.PatchCollector.Create(dexGroup, object_patch.IgnoreIfExists())
	}

	input.PatchCollector.Create(&corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "user-authn-groups-migrated",
			Namespace: "d8-system",
		},
	}, object_patch.IgnoreIfExists())

	return nil
}

const (
	maxMetadataNameLength  = 63
	hashLength             = 10
	maxGeneratedNameLength = maxMetadataNameLength - hashLength - 1 // -1 is for the delimiter
)

const metadataNamePattern = "^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$"

var metadataNameRe = regexp.MustCompile(metadataNamePattern)

func metadataNameify(base string) string {
	// if the name valid, we want to preserve it to not punish users with groups named properly
	if metadataNameRe.MatchString(base) && len(base) <= maxMetadataNameLength {
		return base
	}

	// This is required to avoid collisions when sanitized group names are equal, e.g., Admins and admins.
	// Must go first before all changes.
	hash := fnv.New32a()
	hash.Write([]byte(base))

	base = strings.ToLower(base)

	// Only a-z, 0-9, . and - are allowed
	runes := []rune(base)
	for i, c := range runes {
		alpha := c >= 'a' && c <= 'z'
		digit := c >= '0' && c <= '9'
		delimiter := c == '-'

		if !alpha && !digit && !delimiter {
			runes[i] = '-'
		}
	}
	base = string(runes)

	base = strings.Trim(base, "-")

	if len(base) > maxGeneratedNameLength {
		base = base[:maxGeneratedNameLength]
	}

	base = fmt.Sprintf("%s-%d", base, hash.Sum32())

	return base
}
