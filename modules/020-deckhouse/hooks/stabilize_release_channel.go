/*
Copyright 2021 Flant CJSC

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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/deckhouse/stabilize_release_channel",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "stabilize_release_channel",
			Crontab: "*/10 * * * *",
		},
	},
}, dependency.WithExternalDependencies(setReleaseChannel))

func setReleaseChannel(input *go_hook.HookInput, dc dependency.Container) error {
	const (
		registryKey       = "global.modulesImages.registry"
		releaseChannelKey = "deckhouse.releaseChannel"
		imageKey          = "deckhouse.internal.currentReleaseImageName" // full image name host/ns/repo:tag
	)
	var (
		repo                      = input.Values.Get(registryKey).String()
		currentImageTag           = input.Values.Get(imageKey).String()
		desiredReleaseChannelName = input.Values.Get(releaseChannelKey).String()
		desiredReleaseChannel     = releaseChannelFromName(desiredReleaseChannelName)
	)

	// Check desired release channel
	if desiredReleaseChannelName == "" {
		return nil
	}
	if !desiredReleaseChannel.IsKnown() {
		return fmt.Errorf("invalid desired release channel, check 'deckhouse.releaseChannel' in deckhouse configmap")
	}

	// Check current release channel
	currentReleaseChannel, isKnown := getCurrentChannel(currentImageTag, repo)
	if !isKnown {
		// Current image tag does not match any release channel, cannot stabilize.
		return nil
	}

	// Should we do anything?
	if desiredReleaseChannel == currentReleaseChannel {
		return nil
	}

	registry, err := dc.GetRegistryClient(repo)
	if err != nil {
		return fmt.Errorf("cannot init registry client: %v", err)
	}

	currentDigest, err := registry.Digest(currentReleaseChannel.Tag())
	if err != nil {
		return fmt.Errorf("cannot obtain image digest: %v", err)
	}

	// Choose new image to come closer to the desired release channel BY ONE.

	var (
		newReleaseChannel = currentReleaseChannel
		newImageTag       = currentImageTag
	)

	if desiredReleaseChannel < currentReleaseChannel {
		// Decrease the release channel stability to have newer version.
		for newReleaseChannel > minReleaseChannel {
			newReleaseChannel--
			tag := repo + ":" + newReleaseChannel.Tag()
			digest, err := registry.Digest(newReleaseChannel.Tag())
			if err != nil {
				return fmt.Errorf("cannot obtain image digest: %v", err)
			}
			if currentDigest != digest || newReleaseChannel == desiredReleaseChannel {
				newImageTag = tag
				break
			}
		}
	} else {
		// Increase the release channel stability and wait for it.
		for newReleaseChannel < maxReleaseChannel {
			newReleaseChannel++
			tag := repo + ":" + newReleaseChannel.Tag()
			digest, err := registry.Digest(newReleaseChannel.Tag())
			if err != nil {
				return fmt.Errorf("cannot obtain image digest: %v", err)
			}
			if currentDigest != digest {
				// The release cannot downgrade, it can switch if the image appears to
				// be the same for the channels
				break
			}
			newImageTag = tag
		}
	}

	if newImageTag != currentImageTag {
		input.Values.Set(imageKey, newImageTag)
	}

	return nil
}

func getCurrentChannel(currentImageTag, repo string) (releaseChannel, bool) {
	parts := strings.Split(currentImageTag, ":")
	if len(parts) != 2 || parts[0] != repo {
		return unknownReleaseChannel, false
	}
	currentReleaseChannelName := parts[1]
	currentReleaseChannel := releaseChannelFromName(currentReleaseChannelName)
	return currentReleaseChannel, currentReleaseChannel.IsKnown()
}

const (
	unknownReleaseChannel releaseChannel = iota - 1
	alphaReleaseChannel
	betaReleaseChannel
	earlyAccessReleaseChannel
	stableReleaseChannel
	rockSolidReleaseChannel
)

// Known release channels

const (
	tagAlpha       = "alpha"
	tagBeta        = "beta"
	tagEarlyAccess = "early-access"
	tagStable      = "stable"
	tagRockSolid   = "rock-solid"

	nameAlpha       = "Alpha"
	nameBeta        = "Beta"
	nameEarlyAccess = "EarlyAccess"
	nameStable      = "Stable"
	nameRockSolid   = "RockSolid"

	minReleaseChannel = alphaReleaseChannel
	maxReleaseChannel = rockSolidReleaseChannel
)

type releaseChannel int

func (r releaseChannel) String() string {
	switch r {
	case alphaReleaseChannel:
		return nameAlpha
	case betaReleaseChannel:
		return nameBeta
	case earlyAccessReleaseChannel:
		return nameEarlyAccess
	case stableReleaseChannel:
		return nameStable
	case rockSolidReleaseChannel:
		return nameRockSolid
	default:
		return ""
	}
}

func (r releaseChannel) Tag() string {
	return name2tag(r.String())
}

func (r releaseChannel) IsKnown() bool {
	return r >= minReleaseChannel && r <= maxReleaseChannel
}

func name2tag(s string) string {
	switch s {
	case nameAlpha:
		return tagAlpha
	case nameBeta:
		return tagBeta
	case nameEarlyAccess:
		return tagEarlyAccess
	case nameStable:
		return tagStable
	case nameRockSolid:
		return tagRockSolid
	default:
		return s
	}
}

func releaseChannelFromName(s string) releaseChannel {
	switch name2tag(s) {
	case tagAlpha:
		return alphaReleaseChannel
	case tagBeta:
		return betaReleaseChannel
	case tagEarlyAccess:
		return earlyAccessReleaseChannel
	case tagStable:
		return stableReleaseChannel
	case tagRockSolid:
		return rockSolidReleaseChannel
	default:
		return unknownReleaseChannel
	}
}
