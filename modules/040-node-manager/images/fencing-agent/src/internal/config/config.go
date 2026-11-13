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
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

// The ConfigMap with the agent configuration.
const (
	ConfigMapNamespace = "d8-cloud-instance-manager"
	ConfigMapName      = "fencing-agent-config"
	ConfigMapKey       = "config.json"
)

// Modes of a node group.
const (
	ModeDisabled = "Disabled"
	ModeNormal   = "Normal"
	ModeSlow     = "Slow"
	ModeCritical = "Critical"
)

// Bounds and defaults of the values that can be out of range.
const (
	DefaultMinFullPollInterval     = 10 * time.Second
	LowestMinFullPollInterval      = 5 * time.Second
	HighestMinFullPollInterval     = 60 * time.Second
	DefaultPeerListRefreshInterval = 5 * time.Minute
	// MinPollBudget is the lower bound of 2 * roundTimeout + confirmTimeout.
	MinPollBudget = 600 * time.Millisecond
)

// Names of the values in the config_invalid metric.
const (
	ParamConfig                  = "config"
	ParamClusterUUID             = "clusterUUID"
	ParamMemberlistProfile       = "memberlistProfile"
	ParamMinFullPollInterval     = "minFullPollInterval"
	ParamPeerListRefreshInterval = "peerListRefreshInterval"
	ParamMode                    = "mode"
	ParamProfile                 = "profile"
	ParamCommonTimings           = "commonTimings"
)

// Params lists every name that the config_invalid metric can have.
var Params = []string{
	ParamConfig, ParamClusterUUID, ParamMemberlistProfile, ParamMinFullPollInterval,
	ParamPeerListRefreshInterval, ParamMode, ParamProfile, ParamCommonTimings,
}

// Config is the value of ConfigMapKey. The field names are the same as in
// templates/fencing-agent/configmap.yaml.
type Config struct {
	// ClusterUUID is the memberlist label.
	ClusterUUID string               `json:"clusterUUID"`
	NodeGroups  map[string]NodeGroup `json:"nodeGroups"`
	// MemberlistProfile is the name of the profile for the memberlist timings.
	MemberlistProfile string `json:"memberlistProfile"`
	// SystemNetworkName is empty when the fallback network is off.
	SystemNetworkName       string          `json:"systemNetworkName"`
	MinFullPollInterval     metav1.Duration `json:"minFullPollInterval"`
	PeerListRefreshInterval metav1.Duration `json:"peerListRefreshInterval"`
	// ProfileChecksums maps a profile name to the checksum annotation that
	// the profile object must have.
	ProfileChecksums map[string]string `json:"profileChecksums"`
	// CommonTimings are the same for all node groups.
	CommonTimings CommonTimings `json:"commonTimings"`
}

// CommonTimings are the timings of the agent that do not depend on the profile.
type CommonTimings struct {
	// RoundTimeout is the timeout of one round of the poll.
	RoundTimeout metav1.Duration `json:"roundTimeout"`
	// ConfirmTimeout is the timeout of the repeated question to the silent peers.
	ConfirmTimeout metav1.Duration `json:"confirmTimeout"`
	// FastReadAfter is the age of the last ready probe after which the agent
	// reads /upstreams more frequently.
	FastReadAfter metav1.Duration `json:"fastReadAfter"`
}

// valid tells whether all timings are positive and the poll budget
// 2 * RoundTimeout + ConfirmTimeout is at least MinPollBudget.
func (t CommonTimings) valid() bool {
	return t.RoundTimeout.Duration > 0 && t.ConfirmTimeout.Duration > 0 && t.FastReadAfter.Duration > 0 &&
		2*t.RoundTimeout.Duration+t.ConfirmTimeout.Duration >= MinPollBudget
}

type NodeGroup struct {
	// Mode is Disabled, Normal, Slow or Critical.
	Mode string `json:"mode"`
	// Engine is true for a group on Deckhouse Engine (systemType Immutable).
	Engine bool `json:"engine"`
}

// Parse decodes the value of ConfigMapKey. It ignores unknown keys: they can come
// from a newer version of the module. It does not validate the values.
func Parse(data []byte) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode %s: %w", ConfigMapKey, err)
	}

	return cfg, nil
}

// ValidMode tells whether mode is one of the modes of a node group.
func ValidMode(mode string) bool {
	return slices.Contains([]string{ModeDisabled, ModeNormal, ModeSlow, ModeCritical}, mode)
}

// ModeOf returns the mode of a node group. A group that is not in the
// ConfigMap is Disabled.
func (c Config) ModeOf(group string) string {
	ng, ok := c.NodeGroups[group]
	if !ok {
		return ModeDisabled
	}

	return ng.Mode
}

// EngineOf tells whether a node group runs on Deckhouse Engine. A group that
// is not in the ConfigMap does not.
func (c Config) EngineOf(group string) bool {
	return c.NodeGroups[group].Engine
}

// Resolve replaces each value that is out of range with its value in prev and
// returns the parameters that it replaced. A value without a valid prev stays
// empty, see Complete.
func (c Config) Resolve(prev Config) (Config, []string) {
	var invalid []string

	if c.ClusterUUID == "" {
		c.ClusterUUID = prev.ClusterUUID
		invalid = append(invalid, ParamClusterUUID)
	}

	if !slices.Contains(profileNames(), c.MemberlistProfile) {
		c.MemberlistProfile = prev.MemberlistProfile
		invalid = append(invalid, ParamMemberlistProfile)
	}

	if c.MinFullPollInterval.Duration < LowestMinFullPollInterval || c.MinFullPollInterval.Duration > HighestMinFullPollInterval {
		c.MinFullPollInterval = prev.MinFullPollInterval
		invalid = append(invalid, ParamMinFullPollInterval)
	}

	if c.PeerListRefreshInterval.Duration < 0 {
		c.PeerListRefreshInterval = prev.PeerListRefreshInterval
		invalid = append(invalid, ParamPeerListRefreshInterval)
	}

	if !c.CommonTimings.valid() {
		c.CommonTimings = prev.CommonTimings
		invalid = append(invalid, ParamCommonTimings)
	}

	return c, invalid
}

// Complete tells whether the values that have no default are set.
func (c Config) Complete() bool {
	return c.ClusterUUID != "" && c.MemberlistProfile != "" && c.CommonTimings.valid()
}

// Defaults returns the Config that Resolve falls back to before the first valid one.
func Defaults() Config {
	return Config{
		MinFullPollInterval:     metav1.Duration{Duration: DefaultMinFullPollInterval},
		PeerListRefreshInterval: metav1.Duration{Duration: DefaultPeerListRefreshInterval},
	}
}

func profileNames() []string {
	names := make([]string, 0, len(v1alpha1.ProfileNames()))
	for _, p := range v1alpha1.ProfileNames() {
		names = append(names, p.ObjectName())
	}

	return names
}

// ProfileName returns the name of the FencingSLAProfile of a mode. A group
// without a watchdog has no profile of its own: it uses the memberlist profile.
func (c Config) ProfileName(mode string) string {
	if mode == ModeDisabled {
		return c.MemberlistProfile
	}

	return strings.ToLower(mode)
}

// Settings is the valid configuration that the agent applies as one unit.
type Settings struct {
	Config Config
	// NodeGroup is the value of the node group label of the own Node.
	NodeGroup string
	// Mode is the mode of NodeGroup.
	Mode string
	// Engine tells whether NodeGroup runs on Deckhouse Engine.
	Engine bool
	// ProfileName is the name of the FencingSLAProfile of Mode. For ModeDisabled
	// it is Config.MemberlistProfile.
	ProfileName string
	Profile     v1alpha1.FencingSLAProfileSpec
	// Memberlist is the FencingSLAProfile named by Config.MemberlistProfile.
	Memberlist v1alpha1.FencingSLAProfileSpec
}
