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

package hooks_configuration

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"
	"github.com/tidwall/gjson"
	"sigs.k8s.io/yaml"
)

type Hook struct {
	Path       string
	Executable bool
	HookConfig HookConfig
	Session    *gexec.Session
}

type HookConfig struct {
	JSON string
}

func (hc *HookConfig) Get(path string) gjson.Result {
	return gjson.Get(hc.JSON, path)
}

func (hc *HookConfig) Parse() gjson.Result {
	return gjson.Parse(hc.JSON)
}

func (hc *HookConfig) Array() []gjson.Result {
	return gjson.Parse(hc.JSON).Array()
}

func (hc *HookConfig) String() string {
	return hc.JSON
}

// FIXME use addon-operator’s methods to discover all hooks.
func GetAllHooks() ([]Hook, error) {
	hooks := []Hook{}

	var hookDirs []string
	for _, possibleDir := range []string{
		"/deckhouse/modules/*/hooks",
		"/deckhouse/ee/modules/*/hooks",
		"/deckhouse/ee/fe/modules/*/hooks",
	} {
		result, err := filepath.Glob(possibleDir)
		if err != nil {
			return []Hook{}, err
		}

		hookDirs = append(hookDirs, result...)
	}

	hookDirs = append(hookDirs, "/deckhouse/global-hooks")

	for _, dir := range hookDirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			switch {
			case err != nil:
				return err
			case strings.Contains(path, "testdata"): // ignore tests
				return nil
			case strings.HasSuffix(path, "test.go"): // ignore tests
				return nil
			case strings.HasSuffix(path, ".go"): // ignore go-hooks
				return nil
			case strings.HasSuffix(path, ".yaml"): // ignore openapi schemas
				return nil
			case strings.HasSuffix(path, ".json"): // ignore json files (golden tests for example)
				return nil
			case strings.HasSuffix(path, ".txt"): // ignore txt files
				return nil
			case strings.HasSuffix(path, ".md"): // ignore Markdown files
				return nil
			case info.IsDir():
				return nil
			}

			// Is hook executable.
			executable := info.Mode()&0111 == 0111

			hooks = append(hooks, Hook{Path: path, Executable: executable})
			return nil
		})
	}
	return hooks, nil
}

func (h *Hook) ExecuteGetConfig() error {
	var (
		hookEnvs        []string
		err             error
		parsedConfig    json.RawMessage
		configJSONBytes []byte
	)

	hookEnvs = append(hookEnvs, "ADDON_OPERATOR_NAMESPACE=tests", "DECKHOUSE_POD=tests", "D8_IS_TESTS_ENVIRONMENT=yes", "PATH="+os.Getenv("PATH"))

	hookCmd := &exec.Cmd{
		Path: h.Path,
		Args: []string{h.Path, "--config"},
		Env:  append(os.Environ(), hookEnvs...),
	}

	h.Session, err = gexec.Start(hookCmd, nil, GinkgoWriter)
	if err != nil {
		return err
	}

	h.Session.Wait(10)
	if h.Session.ExitCode() != 0 {
		return fmt.Errorf("hook execution failed with exit code %d", h.Session.ExitCode())
	}

	out := h.Session.Out.Contents()

	err = yaml.Unmarshal(out, &parsedConfig)
	if err != nil {
		return err
	}

	configJSONBytes, err = parsedConfig.MarshalJSON()
	if err != nil {
		return err
	}

	h.HookConfig.JSON = string(configJSONBytes)

	return nil
}
