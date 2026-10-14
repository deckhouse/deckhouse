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

package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTheFirstMasterInstallAddsNothingToANodeScript(t *testing.T) {
	files := &Files{text: map[string]string{
		"lib.sh.tpl":                        liveTemplate(t, "candi/bashible/lib.sh.tpl"),
		"01-bootstrap-prerequisites.sh.tpl": liveTemplate(t, "candi/bashible/bootstrap/01-bootstrap-prerequisites.sh.tpl"),
		"bb_node_ip.sh.tpl":                 liveTemplate(t, "candi/bashible/bb_node_ip.sh.tpl"),
		"nodelet_install.sh.tpl":            liveTemplate(t, "candi/bashible/nodelet_install.sh.tpl"),
	}}
	in := staticInput(files)
	in.Images = map[string]any{"registrypackages": map[string]any{
		"jq": "sha256:jq", "d8Curl": "sha256:curl", "tailLog": "sha256:tail", "rppGet": "sha256:rpp",
		"nodelet": "sha256:nodelet",
	}}
	with, err := RenderScript(in)
	require.NoError(t, err)

	delete(files.text, "nodelet_install.sh.tpl")
	without, err := RenderScript(in)
	require.NoError(t, err)

	require.Equal(t, string(without), string(with))
	require.NotContains(t, string(with), "bb-d8-node-name() {")
}
