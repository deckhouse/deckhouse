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
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestReleaseVersionDataDecode(t *testing.T) {
	const versionJSON = `{
		"version": "v1.16.1",
		"canary": {
			"stable": {"enabled": true, "waves": 6, "interval": "30m"},
			"alpha": {"enabled": false, "waves": 2, "interval": 300000000000}
		},
		"requirements": {"k8s": "1.19", "req1": "dep1"},
		"disruptions": {"1.16": ["ingressNginx", "cniCilium"]},
		"suspend": true
	}`

	var data ReleaseVersionData
	require.NoError(t, json.Unmarshal([]byte(versionJSON), &data))

	require.Equal(t, "v1.16.1", data.Version)
	require.Equal(t, map[string]string{"k8s": "1.19", "req1": "dep1"}, data.Requirements)
	require.Equal(t, map[string][]string{"1.16": {"ingressNginx", "cniCilium"}}, data.Disruptions)
	require.True(t, data.Suspend)

	// interval accepts both the "30m" form used in release.yaml and a raw nanosecond count
	require.Equal(t, 30*time.Minute, data.Canary["stable"].Interval.Duration)
	require.Equal(t, 5*time.Minute, data.Canary["alpha"].Interval.Duration)

	meta := data.ToMetadata()

	require.Equal(t, &ReleaseMetadata{
		Version: "v1.16.1",
		Canary: map[string]CanarySettings{
			"stable": {Enabled: true, Waves: 6, Interval: 30 * time.Minute},
			"alpha":  {Enabled: false, Waves: 2, Interval: 5 * time.Minute},
		},
		Requirements: map[string]string{"k8s": "1.19", "req1": "dep1"},
		Disruptions:  map[string][]string{"1.16": {"ingressNginx", "cniCilium"}},
		Suspend:      true,
	}, meta)

	// the changelog comes from another file of the release image, never from version.json
	require.Nil(t, meta.Changelog)
}

func TestReleaseVersionDataToMetadataEmpty(t *testing.T) {
	meta := new(ReleaseVersionData).ToMetadata()

	require.Equal(t, &ReleaseMetadata{}, meta)
	require.Nil(t, meta.Canary)
}

func TestIsCanaryRelease(t *testing.T) {
	meta := &ReleaseMetadata{
		Canary: map[string]CanarySettings{
			"stable":       {Enabled: true, Waves: 6, Interval: 30 * time.Minute},
			"beta":         {Enabled: false, Waves: 1, Interval: time.Minute},
			"early-access": {Enabled: true, Waves: 0, Interval: 30 * time.Minute},
		},
	}

	for _, tc := range []struct {
		channel string
		want    bool
	}{
		{channel: "stable", want: true},
		{channel: "beta", want: false},
		// canary is on, but without waves there is nothing to spread the rollout over
		{channel: "early-access", want: false},
		{channel: "rock-solid", want: false},
	} {
		t.Run(tc.channel, func(t *testing.T) {
			require.Equal(t, tc.want, meta.IsCanaryRelease(tc.channel))
		})
	}
}

func TestCalculateReleaseDelay(t *testing.T) {
	ts := metav1.Date(2019, time.October, 17, 15, 33, 0, 0, time.UTC)

	meta := &ReleaseMetadata{
		Version: "v1.16.1",
		Canary: map[string]CanarySettings{
			"stable":       {Enabled: true, Waves: 6, Interval: 30 * time.Minute},
			"alpha":        {Enabled: true, Waves: 2, Interval: 5 * time.Minute},
			"early-access": {Enabled: true, Waves: 0, Interval: 30 * time.Minute},
		},
	}

	for _, tc := range []struct {
		name        string
		channel     string
		clusterUUID string
		want        *metav1.Time
	}{
		{
			name:        "wave 1 of 6 postpones by a single interval",
			channel:     "stable",
			clusterUUID: "cluster-a",
			want:        ptr.To(metav1.Date(2019, time.October, 17, 16, 3, 0, 0, time.UTC)),
		},
		{
			name:        "last wave postpones by the whole rollout",
			channel:     "stable",
			clusterUUID: "cluster-c",
			want:        ptr.To(metav1.Date(2019, time.October, 17, 18, 3, 0, 0, time.UTC)),
		},
		{
			name:        "wave 0 deploys right away",
			channel:     "alpha",
			clusterUUID: "cluster-f",
			want:        nil,
		},
		{
			name:        "channel without waves deploys right away",
			channel:     "early-access",
			clusterUUID: "cluster-a",
			want:        nil,
		},
		{
			name:        "unknown channel deploys right away",
			channel:     "rock-solid",
			clusterUUID: "cluster-a",
			want:        nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, meta.CalculateReleaseDelay(tc.channel, ts, tc.clusterUUID))
		})
	}
}
