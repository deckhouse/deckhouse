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

package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// The label this hook is built on is written by a bashible step, on the node, in another repository
// directory — so the one thing a unit test here can guard is that the two spellings agree, and that the
// filter reports the node by name rather than by anything it would have to interpret.
func TestTheForeignConfigFilterReportsTheNodeName(t *testing.T) {
	node := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Node",
		"metadata": map[string]interface{}{
			"name": "master-0",
			"labels": map[string]interface{}{
				ForeignConfigLabel: ForeignConfigLabelValue,
			},
		},
	}}

	result, err := filterForeignConfigNode(node)
	require.NoError(t, err)
	assert.Equal(t, "master-0", result)
}

// The label's spelling is a contract with candi, and a typo on either side turns this warning into
// silence — which is the failure mode the warning exists to remove. Pinned to the literal strings the
// step writes, so a rename has to be made in both places or fail here.
func TestTheLabelIsSpelledTheWayBashibleWritesIt(t *testing.T) {
	assert.Equal(t, "node.deckhouse.io/containerd-config-registry", ForeignConfigLabel,
		"candi/bashible/common-steps/all/091_check_containerd_conf.d.sh.tpl writes this")
	assert.Equal(t, "custom", ForeignConfigLabelValue,
		"the same step writes `custom` when a conf.d file carries registry fields, `default` otherwise")
}
