package hooks

import (
	"encoding/binary"
	"math/rand"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:     moduleQueue,
	OnStartup: &go_hook.OrderedConfig{Order: 10},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "every_hour",
			Crontab: "11 * * * *",
		},
	},
}, handleRolloutEpoch)

func handleRolloutEpoch(input *go_hook.HookInput) error {
	clusterUUID := input.Values.Get("global.discovery.clusterUUID").String()

	var seed = binary.BigEndian.Uint64([]byte(clusterUUID))
	rand.Seed(int64(seed))

	epoch := ((rand.Int63() * 864000) + time.Now().Unix()) / 864000

	input.Values.Set("controlPlaneManager.internal.rolloutEpoch", epoch)

	return nil
}
