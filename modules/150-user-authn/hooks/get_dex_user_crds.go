/*
Copyright 2021 Flant JSC

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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
)

type expirePatch struct {
	ExpireAt string `json:"expireAt"`
}

type DexUser struct {
	Name        string `json:"name"`
	EncodedName string `json:"encodedName"`

	Spec   map[string]interface{} `json:"spec"`
	Status map[string]interface{} `json:"status,omitempty"`

	ExpireAt string `json:"-"`
}

func applyDexUserFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	spec, ok, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("cannot get spec from dex user: %v", err)
	}
	if !ok {
		return nil, fmt.Errorf("dex user has no spec field")
	}

	status, _, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil {
		return nil, fmt.Errorf("cannot get status from dex user: %v", err)
	}

	name := obj.GetName()

	if _, ok := spec["userID"]; !ok {
		spec["userID"] = name
	}

	var encodedName string
	if email, ok := spec["email"]; ok {
		convertedEmail := email.(string)
		spec["email"] = convertedEmail
		encodedName = encoding.ToFnvLikeDex(strings.ToLower(convertedEmail))
	}

	var expireAt string

	_, ok = status["expireAt"]
	if !ok {
		ttl, ok := spec["ttl"]
		if ok {
			duration, ok := ttl.(string)
			if !ok {
				return nil, fmt.Errorf("cnnot conever ttl to time duration")
			}

			parsedDuration, err := time.ParseDuration(duration)
			if err != nil {
				return nil, fmt.Errorf("cannot parse expiration duration: %v", err)
			}

			expireAt = time.Now().Add(parsedDuration).Format(time.RFC3339)
			delete(spec, "ttl")
		}
	}

	return DexUser{
		Name:        name,
		EncodedName: encodedName,
		Spec:        spec,
		Status:      status,
		ExpireAt:    expireAt,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/user-authn",
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
	},
}, getDexUsers)

func getDexUsers(input *go_hook.HookInput) error {
	users := make([]DexUser, 0, len(input.Snapshots["users"]))

	for _, user := range input.Snapshots["users"] {
		dexUser, ok := user.(DexUser)
		if !ok {
			return fmt.Errorf("cannot convert user to dex user")
		}

		users = append(users, dexUser)
		if dexUser.ExpireAt == "" {
			continue
		}

		patch := map[string]interface{}{
			"status": expirePatch{
				ExpireAt: dexUser.ExpireAt,
			},
		}

		input.PatchCollector.MergePatch(patch, "deckhouse.io/v1", "User", "", dexUser.Name, object_patch.WithSubresource("/status"))
	}

	input.Values.Set("userAuthn.internal.dexUsersCRDs", users)
	return nil
}
