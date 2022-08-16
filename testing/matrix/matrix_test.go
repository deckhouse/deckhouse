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

package matrix

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/testing/matrix/linter"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/modules"
)

func TestMatrix(t *testing.T) {
	// Use environment variable to focus on specific module, e.g. FOCUS=user-authn,user-authz
	focus := os.Getenv("FOCUS")

	focusNames := make(map[string]struct{})
	if focus != "" {
		parts := strings.Split(focus, ",")
		for _, part := range parts {
			focusNames[part] = struct{}{}
		}
	}

	discoveredModules, err := modules.GetDeckhouseModulesWithValuesMatrixTests(focusNames)
	require.NoError(t, err)

	for _, module := range discoveredModules {
		_, ok := focusNames[module.Name]
		if len(focusNames) == 0 || ok {
			t.Run(module.Name, func(t *testing.T) {
				require.NoError(t, linter.Run("", module))
			})
		}
	}
}
