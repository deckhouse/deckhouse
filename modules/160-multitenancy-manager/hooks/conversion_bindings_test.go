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

package hooks

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// conversionConfig is the part of a hook's __config__ that webhook-handler reads to register the
// conversions of a CRD.
type conversionConfig struct {
	ConfigVersion string              `json:"configVersion"`
	Bindings      []conversionBinding `json:"kubernetesCustomResourceConversion"`
}

type conversionBinding struct {
	Name        string           `json:"name"`
	CRDName     string           `json:"crdName"`
	Conversions []conversionPair `json:"conversions"`
}

type conversionPair struct {
	FromVersion string `json:"fromVersion"`
	ToVersion   string `json:"toVersion"`
}

// conversionHandler matches the functions shell-operator calls for a binding, __on_conversion::<name>.
var conversionHandler = regexp.MustCompile(`(?m)^function __on_conversion::(\w+)\(\) \{$`)

// webhook-handler converts only what the __config__ of a hook registers: a binding names the CRD and
// the pair of versions, and shell-operator calls the function __on_conversion::<binding>. The fixtures
// pin what each function does, so every single-step path they hold has to be a binding of the right CRD
// with a function of that name, and the hook has no binding or function beyond them.
func TestConversionBindings(t *testing.T) {
	t.Parallel()

	for _, hook := range []string{conversionHook, templateConversionHook} {
		resource := filepath.Base(hook)
		t.Run(resource, func(t *testing.T) {
			t.Parallel()

			paths := singleStepFixtures(t, filepath.Join(conversionFixtures, resource))
			config := conversionConfigOf(t, hook)
			assert.Equal(t, "v1", config.ConfigVersion)

			bindings := make([]string, 0, len(config.Bindings))
			for _, binding := range config.Bindings {
				bindings = append(bindings, binding.Name)
				assert.Equal(t, resource+".deckhouse.io", binding.CRDName, binding.Name)

				from, to, _ := strings.Cut(binding.Name, "_to_")
				want := []conversionPair{{FromVersion: "deckhouse.io/" + from, ToVersion: "deckhouse.io/" + to}}
				assert.Equal(t, want, binding.Conversions, binding.Name)
			}
			assert.ElementsMatch(t, paths, bindings, "the bindings are the single-step paths of the fixtures")
			assert.ElementsMatch(t, paths, conversionHandlers(t, hook), "every binding has its function and no function is left over")
		})
	}
}

// singleStepFixtures lists the conversion paths of one step that the fixtures of a hook hold. A longer
// path is a chain of them.
func singleStepFixtures(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	var paths []string
	for _, entry := range entries {
		path, ok := strings.CutSuffix(entry.Name(), ".yaml")
		if ok && strings.Count(path, "_to_") == 1 {
			paths = append(paths, path)
		}
	}
	require.NotEmpty(t, paths, "%s holds no conversion of one step", dir)

	return paths
}

// conversionConfigOf decodes the document the __config__ function of a hook prints, a heredoc that ends
// at the first EOF line.
func conversionConfigOf(t *testing.T, hookPath string) conversionConfig {
	t.Helper()

	hook, err := os.ReadFile(hookPath)
	require.NoError(t, err)

	_, body, found := strings.Cut(string(hook), "function __config__() {\n  cat <<EOF\n")
	require.True(t, found, "%s has no __config__ heredoc", hookPath)
	document, _, found := strings.Cut(body, "\nEOF\n")
	require.True(t, found, "the __config__ heredoc of %s is not closed", hookPath)

	var config conversionConfig
	require.NoError(t, yaml.UnmarshalStrict([]byte(document), &config), "decoding the __config__ of %s", hookPath)

	return config
}

// conversionHandlers lists the binding names a hook has a function for.
func conversionHandlers(t *testing.T, hookPath string) []string {
	t.Helper()

	hook, err := os.ReadFile(hookPath)
	require.NoError(t, err)

	var names []string
	for _, match := range conversionHandler.FindAllStringSubmatch(string(hook), -1) {
		names = append(names, match[1])
	}

	return names
}
