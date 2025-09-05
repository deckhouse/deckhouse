// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	"os"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/pkg/log"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, discoverDeckhouseVersion)

func discoverDeckhouseVersion(_ context.Context, input *go_hook.HookInput) error {
	versionFile := "/deckhouse/version"
	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		versionFile = os.Getenv("D8_VERSION_TMP_FILE")
	}

	version := "unknown"
	content, err := os.ReadFile(versionFile)
	if err != nil {
		input.Logger.Warn("cannot get deckhouse version", log.Err(err))
	} else {
		version = strings.TrimSuffix(string(content), "\n")
	}

	input.Values.Set("global.deckhouseVersion", version)
	return nil
}
