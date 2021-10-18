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
	const imageKey = "deckhouse.internal.currentReleaseImageName" // full image name host/ns/repo:tag

	var (
		currentImageTag           = input.Values.Get(imageKey).String()
		repo                      = input.Values.Get("global.modulesImages.registry").String() // host/ns/repo
		desiredReleaseChannelName = input.Values.Get("deckhouse.releaseChannel").String()
	)

	// Check desired release channel
	if desiredReleaseChannelName == "" {
		input.LogEntry.Debug("desired release channel not set")
		return nil
	}
	desiredReleaseChannel := releaseChannelFromName(desiredReleaseChannelName)
	if !desiredReleaseChannel.IsKnown() {
		return fmt.Errorf("invalid desired release channel name, check 'deckhouse.releaseChannel' in deckhouse configmap")
	}

	// Check current release channel
	currentReleaseChannel, isKnown := parseReleaseChannel(currentImageTag, repo)
	if !isKnown {
		// Current image tag does not match any release channel, cannot stabilize.
		input.LogEntry.Debug("current tag is not from a release channel")
		return nil
	}

	// Should we do anything?
	if desiredReleaseChannel == currentReleaseChannel {
		input.LogEntry.Debugf("current tag %q is the desired tag %q, nothing to do",
			currentReleaseChannel.Tag(), desiredReleaseChannel.Tag())
		return nil
	}

	registry, err := dc.GetRegistryClient(repo, GetCA(input), IsHTTP(input))
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

	if currentReleaseChannel > desiredReleaseChannel {
		// Upgrade, decreasing release channel index
		for newReleaseChannel > desiredReleaseChannel {
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
		input.LogEntry.Debugf("upgrading to %s", newImageTag)
	} else {
		// Downgrade, increasing release channel index
		for newReleaseChannel < desiredReleaseChannel {
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
		input.LogEntry.Debugf("downgrading to %s", newImageTag)
	}

	if newImageTag == currentImageTag {
		input.LogEntry.Warnf("image did not change (%s)", newImageTag)
		return nil
	}

	input.Values.Set(imageKey, newImageTag)
	return nil
}

func parseReleaseChannel(imageTag, repo string) (releaseChannel, bool) {
	imageSplitIndex := strings.LastIndex(imageTag, ":")
	if imageSplitIndex == -1 {
		return unknownReleaseChannel, false
	}
	repoFromImageTag := imageTag[:imageSplitIndex]
	tag := imageTag[imageSplitIndex+1:]
	if repoFromImageTag != repo {
		return unknownReleaseChannel, false
	}
	relChan := releaseChannelFromName(tag)
	return relChan, relChan.IsKnown()
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
