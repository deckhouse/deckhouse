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

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/okmeter/check_release",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "check_okmeter_release",
			Crontab: "* * * * *", // every minute
		},
	},
}, dependency.WithExternalDependencies(checkRelease))

func checkRelease(input *go_hook.HookInput, dc dependency.Container) error {
	repo := input.ConfigValues.Get("okmeter.image.repository").String()
	if repo == "" {
		repo = "registry.okmeter.io/agent/okagent"
	}
	tag := input.ConfigValues.Get("okmeter.image.tag").String()
	if tag == "" {
		tag = "latest"
	}
	regCli, err := dc.GetRegistryClient(repo, cr.WithAuth(""))
	if err != nil {
		return err
	}

	imageHash, err := regCli.Digest(tag)
	if err != nil {
		return err
	}

	previousHash := input.Values.Get("okmeter.internal.currentReleaseImageHash").String()

	if previousHash == imageHash {
		return nil
	}

	currentImage := fmt.Sprintf("%s@%s", repo, imageHash)

	input.Values.Set("okmeter.internal.currentReleaseImage", currentImage)
	input.Values.Set("okmeter.internal.currentReleaseImageHash", imageHash)

	return nil
}
