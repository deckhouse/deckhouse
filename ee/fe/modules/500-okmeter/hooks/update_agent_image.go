/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
		repo = "registry.okmeter.io/agent/okmeter"
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
