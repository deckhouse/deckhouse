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

package config

import (
	"reflect"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// renderedConfig is config.json as templates/fencing-agent/configmap.yaml
// renders it, with a key that the agent does not know.
const renderedConfig = `{
  "clusterUUID": "f49dd1c3-a63a-4565-a06c-625e35587eab",
  "memberlistProfile": "critical",
  "nodeGroups": {"master": {"mode": "Critical", "engine": false}, "system": {"mode": "Disabled", "engine": true}},
  "minFullPollInterval": "10s",
  "peerListRefreshInterval": "0",
  "profileChecksums": {"critical": "c1", "normal": "n1", "slow": "s1"},
  "systemNetworkName": "",
  "commonTimings": {"roundTimeout": "200ms", "confirmTimeout": "700ms", "fastReadAfter": "1.5s"},
  "maxConcurrentEvictions": 1
}`

func TestParseReadsTheRenderedConfig(t *testing.T) {
	cfg, err := Parse([]byte(renderedConfig))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if cfg.ClusterUUID != "f49dd1c3-a63a-4565-a06c-625e35587eab" || cfg.MemberlistProfile != "critical" {
		t.Errorf("cluster UUID %q, memberlist profile %q", cfg.ClusterUUID, cfg.MemberlistProfile)
	}

	if cfg.NodeGroups["master"].Mode != "Critical" || cfg.NodeGroups["system"].Mode != ModeDisabled {
		t.Errorf("node groups %v", cfg.NodeGroups)
	}

	if cfg.EngineOf("master") || !cfg.EngineOf("system") || cfg.EngineOf("absent") {
		t.Errorf("EngineOf does not return the engine flag of the group, or false for an absent group: %v", cfg.NodeGroups)
	}

	if cfg.MinFullPollInterval.Duration != 10*time.Second || cfg.PeerListRefreshInterval.Duration != 0 {
		t.Errorf("minFullPollInterval %s, peerListRefreshInterval %s", cfg.MinFullPollInterval.Duration, cfg.PeerListRefreshInterval.Duration)
	}

	if cfg.ProfileChecksums["normal"] != "n1" {
		t.Errorf("profile checksums %v", cfg.ProfileChecksums)
	}

	if want := (CommonTimings{RoundTimeout: ms(200), ConfirmTimeout: ms(700), FastReadAfter: ms(1500)}); cfg.CommonTimings != want {
		t.Errorf("common timings %+v, want %+v", cfg.CommonTimings, want)
	}
}

func ms(n int) metav1.Duration {
	return metav1.Duration{Duration: time.Duration(n) * time.Millisecond}
}

func TestParseRejectsABadDuration(t *testing.T) {
	if _, err := Parse([]byte(`{"minFullPollInterval": "ten seconds"}`)); err == nil {
		t.Error("Parse accepted minFullPollInterval that is not a duration")
	}
}

func TestLoadEnv(t *testing.T) {
	t.Setenv("NODE_NAME", "worker-1")

	env, err := LoadEnv()
	if err != nil {
		t.Fatalf("LoadEnv: %v", err)
	}

	if env.NodeName != "worker-1" || env.WatchdogDevice != "/dev/watchdog" || env.HealthProbeBindAddress != "127.0.0.1:4250" ||
		env.KeySwitchDelay != 2*time.Minute {
		t.Errorf("env %+v", env)
	}

	t.Setenv("KEY_SWITCH_DELAY", "30s")
	if env, err = LoadEnv(); err != nil || env.KeySwitchDelay != 30*time.Second {
		t.Errorf("KEY_SWITCH_DELAY=30s gives %s, %v", env.KeySwitchDelay, err)
	}
}

func TestLoadEnvRejectsBadValues(t *testing.T) {
	tests := []struct {
		name, key, value, want string
	}{
		{name: "empty node name", key: "NODE_NAME", value: " ", want: "NODE_NAME is empty"},
		{name: "bad log level", key: "LOG_LEVEL", value: "loud", want: "LOG_LEVEL"},
		{name: "zero key switch delay", key: "KEY_SWITCH_DELAY", value: "0s", want: "KEY_SWITCH_DELAY"},
		{name: "negative key switch delay", key: "KEY_SWITCH_DELAY", value: "-1m", want: "KEY_SWITCH_DELAY"},
		{name: "bad key switch delay", key: "KEY_SWITCH_DELAY", value: "two minutes", want: "KEY_SWITCH_DELAY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NODE_NAME", "worker-1")
			t.Setenv(tt.key, tt.value)

			if _, err := LoadEnv(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("LoadEnv error %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func validConfig() Config {
	return Config{
		ClusterUUID:             "f49dd1c3-a63a-4565-a06c-625e35587eab",
		MemberlistProfile:       "normal",
		MinFullPollInterval:     metav1.Duration{Duration: 20 * time.Second},
		PeerListRefreshInterval: metav1.Duration{Duration: time.Minute},
		CommonTimings:           CommonTimings{RoundTimeout: ms(200), ConfirmTimeout: ms(700), FastReadAfter: ms(1500)},
	}
}

func TestResolveKeepsValidValues(t *testing.T) {
	cfg, invalid := validConfig().Resolve(Defaults())

	if len(invalid) != 0 || !reflect.DeepEqual(cfg, validConfig()) {
		t.Errorf("config %+v, invalid %v", cfg, invalid)
	}
}

// The common timings have no default: a config without them is invalid and
// not complete before the first valid one.
func TestConfigWithoutCommonTimingsIsNotComplete(t *testing.T) {
	cfg := validConfig()
	cfg.CommonTimings = CommonTimings{}

	got, invalid := cfg.Resolve(Defaults())
	if len(invalid) != 1 || invalid[0] != ParamCommonTimings || got.Complete() {
		t.Errorf("invalid %v, complete %v; want [%s] and not complete", invalid, got.Complete(), ParamCommonTimings)
	}
}

// The peer list refresh interval of 0 turns the refresh off.
func TestResolveAcceptsZeroRefreshInterval(t *testing.T) {
	cfg := validConfig()
	cfg.PeerListRefreshInterval = metav1.Duration{}

	if _, invalid := cfg.Resolve(Defaults()); len(invalid) != 0 {
		t.Errorf("invalid %v", invalid)
	}
}

func TestResolveFallsBackToThePreviousValue(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(c *Config)
		param  string
	}{
		{"minFullPollInterval below the range", func(c *Config) { c.MinFullPollInterval = metav1.Duration{Duration: 2 * time.Second} }, ParamMinFullPollInterval},
		{"minFullPollInterval above the range", func(c *Config) { c.MinFullPollInterval = metav1.Duration{Duration: 61 * time.Second} }, ParamMinFullPollInterval},
		{"negative refresh interval", func(c *Config) { c.PeerListRefreshInterval = metav1.Duration{Duration: -time.Second} }, ParamPeerListRefreshInterval},
		{"unknown memberlist profile", func(c *Config) { c.MemberlistProfile = "fast" }, ParamMemberlistProfile},
		{"empty cluster UUID", func(c *Config) { c.ClusterUUID = "" }, ParamClusterUUID},
		{"poll budget below 600ms", func(c *Config) { c.CommonTimings.RoundTimeout, c.CommonTimings.ConfirmTimeout = ms(100), ms(399) }, ParamCommonTimings},
		{"no common timings", func(c *Config) { c.CommonTimings = CommonTimings{} }, ParamCommonTimings},
		{"zero fastReadAfter", func(c *Config) { c.CommonTimings.FastReadAfter = ms(0) }, ParamCommonTimings},
		{"negative roundTimeout", func(c *Config) { c.CommonTimings.RoundTimeout = ms(-1) }, ParamCommonTimings},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := validConfig()
			prev.ClusterUUID, prev.MemberlistProfile = "previous", "critical"
			prev.MinFullPollInterval = metav1.Duration{Duration: 30 * time.Second}
			prev.PeerListRefreshInterval = metav1.Duration{Duration: 2 * time.Minute}
			prev.CommonTimings = CommonTimings{RoundTimeout: ms(250), ConfirmTimeout: ms(800), FastReadAfter: ms(2000)}

			cfg := validConfig()
			tt.mutate(&cfg)

			got, invalid := cfg.Resolve(prev)
			if len(invalid) != 1 || invalid[0] != tt.param {
				t.Fatalf("invalid %v, want [%s]", invalid, tt.param)
			}

			// Only the invalid value is replaced.
			want := validConfig()
			switch tt.param {
			case ParamMinFullPollInterval:
				want.MinFullPollInterval = prev.MinFullPollInterval
			case ParamPeerListRefreshInterval:
				want.PeerListRefreshInterval = prev.PeerListRefreshInterval
			case ParamMemberlistProfile:
				want.MemberlistProfile = prev.MemberlistProfile
			case ParamClusterUUID:
				want.ClusterUUID = prev.ClusterUUID
			case ParamCommonTimings:
				want.CommonTimings = prev.CommonTimings
			}

			if !reflect.DeepEqual(got, want) {
				t.Errorf("config %+v, want %+v", got, want)
			}
		})
	}
}

func TestResolveUsesTheDefaultBeforeTheFirstValidValue(t *testing.T) {
	cfg := validConfig()
	cfg.MinFullPollInterval = metav1.Duration{Duration: time.Second}

	got, _ := cfg.Resolve(Defaults())
	if got.MinFullPollInterval.Duration != DefaultMinFullPollInterval {
		t.Errorf("minFullPollInterval is %s, want the default %s", got.MinFullPollInterval.Duration, DefaultMinFullPollInterval)
	}

	cfg.ClusterUUID = ""

	if got, _ := cfg.Resolve(Defaults()); got.Complete() {
		t.Error("a config without a cluster UUID is complete")
	}
}

func TestModeAndProfileName(t *testing.T) {
	cfg := validConfig()
	cfg.NodeGroups = map[string]NodeGroup{"master": {Mode: ModeCritical}, "system": {Mode: ModeDisabled}}

	if cfg.ModeOf("master") != ModeCritical || cfg.ModeOf("system") != ModeDisabled || cfg.ModeOf("absent") != ModeDisabled {
		t.Error("ModeOf does not return the mode of the group, or Disabled for an absent group")
	}

	if cfg.ProfileName(ModeCritical) != "critical" || cfg.ProfileName(ModeSlow) != "slow" {
		t.Error("ProfileName of a mode is not the lower case name")
	}

	// A Disabled group has no profile of its own.
	if cfg.ProfileName(ModeDisabled) != "normal" {
		t.Errorf("ProfileName of Disabled is %q, want the memberlist profile", cfg.ProfileName(ModeDisabled))
	}

	if !ValidMode("Slow") || ValidMode("Watchdog") || ValidMode("") {
		t.Error("ValidMode accepts a wrong mode")
	}
}
