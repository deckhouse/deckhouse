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

const tenDaysInSeconds = 864000

func handleRolloutEpoch(_ context.Context, input *go_hook.HookInput) error {
	clusterUUID := input.Values.Get("global.discovery.clusterUUID").String()

	seed := binary.BigEndian.Uint64([]byte(clusterUUID))
	randomSource := rand.NewSource(int64(seed))
	random := rand.New(randomSource) //nolint:gosec

	epoch := ((int64(random.Uint32()) * tenDaysInSeconds) + time.Now().Unix()) / tenDaysInSeconds

	input.Values.Set("controlPlaneManager.internal.rolloutEpoch", epoch)

	return nil
}
