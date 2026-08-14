// Copyright 2026 Flant JSC
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

package release

import (
	"slices"
	"strings"
)

// Channel is a release channel name. Channels live here because that is what
// they are: tags on a release repository, alongside concrete version tags.
type Channel string

const (
	Alpha       Channel = "alpha"
	Beta        Channel = "beta"
	EarlyAccess Channel = "early-access"
	Stable      Channel = "stable"
	RockSolid   Channel = "rock-solid"
	LTS         Channel = "lts"
)

// knownChannels lists the release channels in ascending order of stability.
var knownChannels = []Channel{Alpha, Beta, EarlyAccess, Stable, RockSolid, LTS}

// AllChannels returns every known release channel, least stable first. It is
// the fixed vocabulary; Service.Channels reports which of them a given
// repository actually publishes.
func AllChannels() []Channel {
	return slices.Clone(knownChannels)
}

// String returns the channel as it appears in a registry tag.
func (c Channel) String() string {
	return string(c)
}

// IsValid reports whether c is a known release channel.
func (c Channel) IsValid() bool {
	return slices.Contains(knownChannels, c)
}

// IsChannel reports whether a tag names a release channel rather than a
// concrete version or digest.
func IsChannel(tag string) bool {
	return Channel(strings.ToLower(tag)).IsValid()
}
