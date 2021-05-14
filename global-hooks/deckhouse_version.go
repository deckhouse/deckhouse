package hooks

import (
	"io/ioutil"
	"os"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, discoverDeckhouseVersion)

func discoverDeckhouseVersion(input *go_hook.HookInput) error {
	versionFile := "/deckhouse/version"
	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		versionFile = os.Getenv("D8_VERSION_TMP_FILE")
	}

	version := "unknown"
	content, err := ioutil.ReadFile(versionFile)
	if err != nil {
		input.LogEntry.Warnf("cannot get deckhouse version: %v", err)
	} else {
		version = string(content)
	}

	input.Values.Set("global.deckhouseVersion", version)
	return nil
}
