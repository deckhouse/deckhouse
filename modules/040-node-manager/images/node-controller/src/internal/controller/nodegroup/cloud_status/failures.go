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

package cloud_status

import (
	"time"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

func ConvertMachineFailures(failures []common.MachineFailure) []v1.MachineFailure {
	if len(failures) == 0 {
		return nil
	}
	result := make([]v1.MachineFailure, 0, len(failures))
	for _, f := range failures {
		mf := v1.MachineFailure{
			Name:       f.MachineName,
			ProviderID: f.ProviderID,
			OwnerRef:   f.OwnerRef,
		}
		if f.Message != "" {
			state := f.State
			if state == "" {
				state = "Failed"
			}
			opType := f.Type
			if opType == "" {
				opType = "Create"
			}
			mf.LastOperation = &v1.MachineLastOperation{
				Description:    f.Message,
				LastUpdateTime: f.Time.Format(time.RFC3339),
				State:          state,
				Type:           opType,
			}
		}
		result = append(result, mf)
	}
	return result
}
