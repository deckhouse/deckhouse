package hooks

import (
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type DexUserExpire struct {
	Name     string    `json:"name"`
	ExpireAt time.Time `json:"expireAt"`

	CheckExpire bool `json:"-"`
}

func (*DexUserExpire) ApplyFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	status, ok, err := unstructured.NestedMap(obj.Object, "status")
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
			Name:       "users",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "User",
			Filterable: &DexUserExpire{},
		},
	},
}, expireDexUsers)

func expireDexUsers(input *go_hook.HookInput) error {
	now := time.Now()

	for _, user := range input.Snapshots["users"] {
		dexUserExpire, ok := user.(DexUserExpire)
		if !ok {
			return fmt.Errorf("cannot convert user to dex expire")
		}

		if dexUserExpire.CheckExpire && dexUserExpire.ExpireAt.Before(now) {
			err := input.ObjectPatcher.DeleteObject(
				/*apiVersion*/ "deckhouse.io/v1alpha1",
				/*kind*/ "User",
				/*namespace*/ "",
				/*name*/ dexUserExpire.Name,
				/*subresource*/ "",
			)
			if err != nil {
				return fmt.Errorf("cannot delete user %s", dexUserExpire.Name)
			}
		}
	}
	return nil
}
