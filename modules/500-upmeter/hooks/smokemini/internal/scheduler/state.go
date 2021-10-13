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

// State is the internal state for smoke-mini
type State map[string]*XState

func (s State) Empty() bool {
	for _, v := range s {
		if !v.empty() {
			return false
		}
	}
	return true
}

// Populate fills the state with actual values. It is used only on the first run, when the
// state is empty. In all other cases the state is the source of truth.
func (s State) Populate(statefulsets []snapshot.StatefulSet) {
	for _, sts := range statefulsets {
		s[sts.Index] = &XState{
			StorageClass: sts.StorageClass,
			Image:        sts.Image,
			Node:         sts.Node,
			Zone:         sts.Zone,
		}
	}
}

// XState is the state of a single StatefulSet
type XState struct {
	Image        string `json:"image,omitempty"`
	Node         string `json:"node,omitempty"`
	Zone         string `json:"zone,omitempty"`
	StorageClass string `json:"effectiveStorageClass,omitempty"`
}

func (s *XState) empty() bool {
	return s.Image == "" &&
		s.Node == "" &&
		s.Zone == "" &&
		s.StorageClass == ""
}

func (s *XState) scheduled() bool {
	return s.Image != "" &&
		s.Node != "" &&
		s.Zone != "" &&
		s.StorageClass != ""
}
