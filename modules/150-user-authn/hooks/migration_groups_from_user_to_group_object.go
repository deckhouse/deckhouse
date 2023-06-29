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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/user-authn/migration",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "users",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "User",
			FilterFunc: applyDexUserFilter,
		},
	},
}, dependency.WithExternalDependencies(migrationGroups))

func migrationGroups(input *go_hook.HookInput, dc dependency.Container) error {
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	const (
		DexGroupKind       = "Group"
		DexGroupGroup      = "deckhouse.io"
		DexGroupVersion    = "v1alpha1"
		DexGroupResource   = "groups"
		DexGroupAPIVersion = "deckhouse.io/v1alpha1"
	)

	gvr := schema.GroupVersionResource{
		Group:    DexGroupGroup,
		Version:  DexGroupVersion,
		Resource: DexGroupResource,
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
		newDexGroup := &DexGroup{
			TypeMeta: metav1.TypeMeta{
				Kind:       DexGroupKind,
				APIVersion: DexGroupAPIVersion,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: strings.ToLower(groupName),
			},
			Spec: DexGroupSpec{
				Name:    groupName,
				Members: members,
			},
		}

		obj, err := sdk.ToUnstructured(newDexGroup)
		if err != nil {
			return fmt.Errorf("converting DexGroup/%s to unstructured: %w", groupName, err)
		}

		input.LogEntry.Printf("Create Group %s with members %s", groupName, members)
		_, err = kubeClient.Dynamic().Resource(gvr).Create(context.TODO(), obj, metav1.CreateOptions{})
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				return fmt.Errorf("create DexGroup %s: %w", groupName, err)
			}
		}
	}

	return nil
}
