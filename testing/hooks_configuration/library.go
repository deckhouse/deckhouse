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
	hookDirs, err := filepath.Glob("/deckhouse/modules/*/hooks")
	if err != nil {
		return []Hook{}, err
	}

	hookDirs = append(hookDirs, "/deckhouse/global-hooks")

	for _, dir := range hookDirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Ignore tests.
			if strings.HasSuffix(path, "test.go") {
				return nil
			}

			// Ignore golang hooks.
			if strings.HasSuffix(path, ".go") {
				return nil
			}

			// Ignore subdirectories.
			if info.IsDir() {
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
