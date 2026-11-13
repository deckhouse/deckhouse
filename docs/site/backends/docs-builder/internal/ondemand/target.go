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

package ondemand

// Target is the version of a module that represents it in the search index
// and llms.txt. The JSON names are those of targets.json; decoding ignores their case, so
// files written before the tags still read.
type Target struct {
	Module  string `json:"module"`
	Version string `json:"version"`
	// Rev changes when the same version name gets new content: the image
	// digest of a development branch. Empty for a release version.
	Rev string `json:"rev,omitempty"`
}

// TargetChannels are tried in this order for a module that is not enabled:
// the most stable one first. rock-solid is left out, as on the module pages.
var TargetChannels = []string{"stable", "early-access", "beta", "alpha"}

// SiteChannels are the release channels with pages of their own on the
// public site, /modules/<module>/<channel>/.
var SiteChannels = []string{"alpha", "beta", "early-access", "stable", "rock-solid"}

// Channels maps the release channels of a module to the versions they point
// at.
type Channels map[string]string

// ByModule maps the targets by their module.
func ByModule(targets []Target) map[string]Target {
	out := make(map[string]Target, len(targets))
	for _, t := range targets {
		out[t.Module] = t
	}

	return out
}
