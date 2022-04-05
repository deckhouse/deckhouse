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

package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"d8.io/upmeter/pkg/check"
)

func Test_newSkippedFilter(t *testing.T) {
	filter := newSkippedFilter([]string{"full/ref", "notslashed", "slashed/"})

	// exact matches
	assert.False(t, filter.Enabled(check.ProbeRef{Group: "full", Probe: "ref"}))
	assert.False(t, filter.Enabled(check.ProbeRef{Group: "notslashed", Probe: ""}))
	assert.False(t, filter.Enabled(check.ProbeRef{Group: "slashed", Probe: ""}))

	// probes under group notations
	assert.False(t, filter.Enabled(check.ProbeRef{Group: "notslashed", Probe: "probe"}))
	assert.False(t, filter.Enabled(check.ProbeRef{Group: "slashed", Probe: "probe"}))

	// not mentioned
	assert.True(t, filter.Enabled(check.ProbeRef{Group: "something", Probe: ""}))
	assert.True(t, filter.Enabled(check.ProbeRef{Group: "something", Probe: "else"}))
}
