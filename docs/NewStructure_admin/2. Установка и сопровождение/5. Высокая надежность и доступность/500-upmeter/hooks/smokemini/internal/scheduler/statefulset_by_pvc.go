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

package scheduler

import (
	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

type selectByPVC struct {
	pvcs []snapshot.PvcTermination
}

func (s *selectByPVC) Select(_ State) (string, error) {
	for _, pvc := range s.pvcs {
		if pvc.IsTerminating {
			return pvc.Index().String(), nil
		}
	}
	return "", errNext
}
