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
	"context"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

type DexUserExpire struct {
	Name     string    `json:"name"`
	ExpireAt time.Time `json:"expireAt"`

	CheckExpire bool `json:"-"`
}

func applyDexUserExpireFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	status, _, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil {
		return nil, fmt.Errorf("cannot get status from dex user: %v", err)
	}

	dexUserExpire := DexUserExpire{Name: obj.GetName()}

	expireAtFromStatus, ok := status["expireAt"]
	if ok {
		convertedExpireAt, ok := expireAtFromStatus.(string)
		if !ok {
			return nil, fmt.Errorf("cannot convert 'expireAt' to string")
		}

		dexUserExpire.ExpireAt, err = time.Parse(time.RFC3339, convertedExpireAt)
		if err != nil {
			return nil, fmt.Errorf("cannot conver expireAt to time")
		}

		dexUserExpire.CheckExpire = true
	}

	return dexUserExpire, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/user-authn",
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "*/5 * * * *"},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "users",
			ApiVersion:                   "deckhouse.io/v1",
			Kind:                         "User",
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   applyDexUserExpireFilter,
		},
	},
}, expireDexUsers)

func expireDexUsers(_ context.Context, input *go_hook.HookInput) error {
	now := time.Now()

	for dexUserExpire, err := range sdkobjectpatch.SnapshotIter[DexUserExpire](input.Snapshots.Get("users")) {
		if err != nil {
			return fmt.Errorf("cannot convert user to dex expire: cannot iterate over 'users' snapshot: %v", err)
		}

		if dexUserExpire.CheckExpire && dexUserExpire.ExpireAt.Before(now) {
			input.PatchCollector.Delete("deckhouse.io/v1", "User", "", dexUserExpire.Name)
		}
	}
	return nil
}
