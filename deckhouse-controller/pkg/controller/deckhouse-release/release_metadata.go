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

package deckhouse_release

import (
	"time"

	"github.com/spaolacci/murmur3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/libapi"
)

// ReleaseVersionData is the wire format of the version.json file shipped inside a
// Deckhouse release image. It exists only to decode the registry payload: past the
// registry boundary the controller works with ReleaseMetadata.
type ReleaseVersionData struct {
	Version      string                       `json:"version"`
	Canary       map[string]CanaryChannelData `json:"canary"`
	Requirements map[string]string            `json:"requirements"`
	Disruptions  map[string][]string          `json:"disruptions"`
	Suspend      bool                         `json:"suspend"`
}

// CanaryChannelData is the canary block of version.json for a single release channel.
type CanaryChannelData struct {
	Enabled  bool            `json:"enabled"`
	Waves    uint            `json:"waves"`
	Interval libapi.Duration `json:"interval"`
}

// ToMetadata converts the decoded version.json into the domain entity.
func (d *ReleaseVersionData) ToMetadata() *ReleaseMetadata {
	meta := &ReleaseMetadata{
		Version:      d.Version,
		Requirements: d.Requirements,
		Disruptions:  d.Disruptions,
		Suspend:      d.Suspend,
	}

	if d.Canary != nil {
		meta.Canary = make(map[string]CanarySettings, len(d.Canary))

		for channel, settings := range d.Canary {
			meta.Canary[channel] = CanarySettings{
				Enabled:  settings.Enabled,
				Waves:    settings.Waves,
				Interval: settings.Interval.Duration,
			}
		}
	}

	return meta
}

// ReleaseMetadata describes a Deckhouse release as assembled from the files of a
// release image: version.json, changelog.yaml and, when the image carries one,
// module.yaml.
type ReleaseMetadata struct {
	Version   string
	Changelog map[string]any

	Canary       map[string]CanarySettings
	Requirements map[string]string
	Disruptions  map[string][]string
	Suspend      bool
}

// CanarySettings is the canary rollout configuration of a single release channel.
type CanarySettings struct {
	Enabled  bool
	Waves    uint
	Interval time.Duration
}

// IsCanaryRelease reports whether canary is switched on for the given channel.
func (m *ReleaseMetadata) IsCanaryRelease(channel string) bool {
	return m.Canary[channel].Enabled
}

// CalculateReleaseDelay spreads clusters over the canary waves of the channel and
// returns the time this cluster may apply the release at, or nil for the first wave.
// Only call it for a channel IsCanaryRelease reports true for.
// https://github.com/deckhouse/deckhouse/issues/332
func (m *ReleaseMetadata) CalculateReleaseDelay(channel string, ts metav1.Time, clusterUUID string) *metav1.Time {
	settings := m.Canary[channel]

	hash := murmur3.Sum64([]byte(clusterUUID + m.Version))

	wave := hash % uint64(settings.Waves)
	if wave == 0 {
		return nil
	}

	applyAfter := metav1.NewTime(ts.Add(time.Duration(wave) * settings.Interval))

	return &applyAfter
}
