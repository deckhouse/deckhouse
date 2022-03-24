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

package entity

import (
	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/db/dao"
)

func FilterDisabledProbesFromGroupProbeList(probeRefs []check.ProbeRef) []check.ProbeRef {
	res := make([]check.ProbeRef, 0)

	for _, probeRef := range probeRefs {
		if !check.IsProbeEnabled(probeRef.Id()) {
			continue
		}
		if probeRef.Probe == dao.GroupAggregation {
			continue
		}
		res = append(res, probeRef)
	}

	return res
}
