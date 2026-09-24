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

package validating_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

const validatingHook = "dex_authenticator"

// hookIPCheck is what is_ip_address runs under python3. The stand-in under testdata emulates
// exactly this, so it is only put on PATH while the hook still asks for it.
const hookIPCheck = "ipaddress.ip_address(sys.argv[1])"

// The tools the hook needs. Each has a stand-in under testdata, built only when the image carries
// no real one; a present tool is preferred, to keep local runs on what production uses.
var hookTools = []string{"jq", "python3"}

// These tests run the hook the way shell-operator does, because it is Bash rather than one jq
// program: the domains it collects, the duplicate check and the choice of message all live in the
// shell, not in a query that could be executed on its own.
func TestMain(m *testing.M) {
	os.Exit(runHookTests(m))
}

func runHookTests(m *testing.M) int {
	if _, err := exec.LookPath("bash"); err != nil {
		fmt.Fprintf(os.Stderr, "bash is required: these tests run %s\n", validatingHook)

		return 1
	}

	dir, err := os.MkdirTemp("", "hookshims")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}
	defer os.RemoveAll(dir)

	for _, tool := range hookTools {
		if _, err := exec.LookPath(tool); err == nil {
			continue
		}
		if tool == "python3" {
			if err := checkIPCheckUnchanged(); err != nil {
				fmt.Fprintln(os.Stderr, err)

				return 1
			}
		}

		source := filepath.Join("testdata", tool)
		build := exec.Command("go", "build", "-o", filepath.Join(dir, tool), "./"+source)
		if out, err := build.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "building the %s stand-in: %v\n%s", tool, err, out)

			return 1
		}
		fmt.Fprintf(os.Stderr, "no %s found, standing in with %s\n", tool, source)
	}

	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	return m.Run()
}

// checkIPCheckUnchanged refuses to emulate Python the hook no longer runs. The hook reads only the
// exit status of python3 and discards its output, so the stand-in itself cannot report this.
func checkIPCheckUnchanged() error {
	hook, err := os.ReadFile(validatingHook)
	if err != nil {
		return fmt.Errorf("reading %s: %w", validatingHook, err)
	}

	if !strings.Contains(string(hook), hookIPCheck) {
		return fmt.Errorf("is_ip_address no longer runs %s, which testdata/python3 emulates: "+
			"revisit the stand-in, or install python3 to run the hook as it is", hookIPCheck)
	}

	return nil
}

// runHook executes the hook, substituting only shell-operator context and response plumbing.
func runHook(t *testing.T, context map[string]any) map[string]any {
	t.Helper()
	script, err := os.ReadFile(validatingHook)
	require.NoError(t, err)

	dir := t.TempDir()
	input, err := json.Marshal(context)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "context.json"), input, 0600))

	source := strings.Replace(string(script), "source /shell_lib.sh", `function context::jq() { jq "$@" "$TEST_CONTEXT"; }
function hook::run() { :; }`, 1)
	cmd := exec.Command("bash", "-e", "-c", source+"\n__main__")
	response := filepath.Join(dir, "response.json")
	cmd.Env = append(os.Environ(), "TEST_CONTEXT="+filepath.Join(dir, "context.json"), "VALIDATING_RESPONSE_PATH="+response)

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	data, err := os.ReadFile(response)
	require.NoError(t, err, "%s", output)

	result := map[string]any{}
	require.NoError(t, json.Unmarshal(data, &result))

	return result
}

// review wraps an object the way the API server presents it to the hook.
func review(object map[string]any, snapshots ...any) map[string]any {
	return map[string]any{
		"review":    map[string]any{"request": map[string]any{"operation": "CREATE", "object": object}},
		"snapshots": map[string]any{"dexauthenticators": append([]any{}, snapshots...)},
	}
}

// snapshot runs the hook's own jqFilter over an object, as shell-operator does when it maintains
// the list of existing authenticators. The watch reads v1, whatever version the author used.
func snapshot(t *testing.T, object map[string]any) map[string]any {
	t.Helper()
	script, err := os.ReadFile(validatingHook)
	require.NoError(t, err)

	source := strings.Replace(string(script), "source /shell_lib.sh", "function hook::run() { :; }", 1)
	configYAML, err := exec.Command("bash", "-e", "-c", source+"\n__config__").CombinedOutput()
	require.NoError(t, err, "%s", configYAML)

	var config struct {
		Kubernetes []struct {
			JQFilter string `json:"jqFilter"`
		} `json:"kubernetes"`
	}
	require.NoError(t, yaml.Unmarshal(configYAML, &config))
	require.Len(t, config.Kubernetes, 1)
	require.NotEmpty(t, config.Kubernetes[0].JQFilter)

	input, err := json.Marshal(object)
	require.NoError(t, err)
	cmd := exec.Command("jq", config.Kubernetes[0].JQFilter)
	cmd.Stdin = strings.NewReader(string(input))

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)

	filtered := map[string]any{}
	require.NoError(t, json.Unmarshal(output, &filtered))

	return map[string]any{"filterResult": filtered}
}
