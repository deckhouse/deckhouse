/*
Copyright 2024 Flant JSC

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

package requirements

import (
	"regexp"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

func TestModuleRegexp(t *testing.T) {
	funcName := "github.com/deckhouse/deckhouse/modules/402-ingress-nginx/requirements.init.0.func1"
	rr := mreg.FindStringSubmatch(funcName)
	assert.Equal(t, "ingress-nginx", rr[2])
}

func TestCheckRequirements(t *testing.T) {
	f := func(requirementValue string, getter ValueGetter) (bool, error) {
		return false, errors.New("mock error")
	}

	// overwrite the regexp
	mreg = regexp.MustCompile(`/go_lib/([0-9]+-)?(\S+)/requirements`)

	RegisterCheck("test-me", f)

	t.Run("module is enabled", func(t *testing.T) {
		s := set.New("dependency")
		pass, err := CheckRequirement("test-me", "test", s)
		assert.False(t, pass)
		assert.ErrorContains(t, err, "mock error")
	})

	t.Run("module is disabled", func(t *testing.T) {
		s := set.New("not-found")
		pass, err := CheckRequirement("test-me", "test", s)
		// should not run the check
		assert.True(t, pass)
		assert.NoError(t, err)
	})
}
