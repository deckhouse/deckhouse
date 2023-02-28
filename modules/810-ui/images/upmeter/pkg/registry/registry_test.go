/*
Copyright 2023 Flant JSC

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

package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"d8.io/upmeter/pkg/check"
)

// Test how the lister deduplicates and sorts groups and probes
func TestNewProbeLister(t *testing.T) {
	listerZA := lister{
		groups: []string{"z", "a"},
		probes: []check.ProbeRef{
			{Group: "z", Probe: "pz"},
			{Group: "a", Probe: "pa"},
			{Group: "z", Probe: "px"},
		},
	}

	listerXO := lister{
		groups: []string{"o", "x", "z"},
		probes: []check.ProbeRef{
			{Group: "x", Probe: "pk"},
			{Group: "o", Probe: "po"},
			{Group: "z", Probe: "zz"},
			{Group: "x", Probe: "px"},
		},
	}

	pl := NewProbeLister(listerZA, listerXO)

	allProbesSorted := []check.ProbeRef{
		{Group: "a", Probe: "pa"},
		{Group: "o", Probe: "po"},
		{Group: "x", Probe: "pk"},
		{Group: "x", Probe: "px"},
		{Group: "z", Probe: "px"},
		{Group: "z", Probe: "pz"},
		{Group: "z", Probe: "zz"},
	}

	allGroupsSorted := []string{"a", "o", "x", "z"}

	assert.Equal(t, allProbesSorted, pl.Probes())
	assert.Equal(t, allGroupsSorted, pl.Groups())
}

type lister struct {
	groups []string
	probes []check.ProbeRef
}

func (l lister) Groups() []string {
	return l.groups
}

func (l lister) Probes() []check.ProbeRef {
	return l.probes
}
