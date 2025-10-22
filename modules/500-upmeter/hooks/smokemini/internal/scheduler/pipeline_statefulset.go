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
	"errors"
	"fmt"

	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

var (
	// ErrSkip is the legal abortion of scheduling
	ErrSkip = fmt.Errorf("scheduling skipped")

	// errNext lets one step in pipeline to pass the control to the next step
	errNext = fmt.Errorf("next step")
)

// NewStatefulSetSelector creates statefulset choosing pipeline. The result returns ffrom the first
// successful selection. If no selection occurs, the pipeline returns ErrSkip.
func NewStatefulSetSelector(
	nodes []snapshot.Node,
	storageClass string,
	pvcs []snapshot.PvcTermination,
	pods []snapshot.Pod,
	disruptionAllowed bool,
) IndexSelectorPipe {
	xSel := IndexSelectorPipe{
		&selectByPVC{pvcs: pvcs},
		&selectByNode{nodes: nodes},
		&selectByStorageClass{storageClass: storageClass},
		&selectByPod{pods: pods, disruptionAllowed: disruptionAllowed},
	}
	return xSel
}

type IndexSelector interface {
	Select(State) (string, error)
}

// IndexSelectorPipe is the sequential wrapper for other sts selectors. The result is returned from the
// first successful selection or abortion error. Selection is ignored on next error.
type IndexSelectorPipe []IndexSelector

func (s IndexSelectorPipe) Select(state State) (string, error) {
	for _, s := range s {
		x, err := s.Select(state)
		if errors.Is(err, errNext) {
			continue
		}
		return x, err
	}
	return "", ErrSkip
}
