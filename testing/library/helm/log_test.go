// Copyright 2021 Flant JSC
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

package helm

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogTrimmer(t *testing.T) {
	testData := []byte(`
walk.go:74: found symbolic link in path: /deckhouse/modules/...
walk.go:74: found symbolic link in path: /deckhouse/modules/...
walk.go:74: found symbolic link in path: /deckhouse/modules/...
walk.go:74: found symbolic link in path: /deckhouse/modules/...
walk.go:74: found symbolic link in path: /deckhouse/modules/...
walk.go:74: found symbolic link in path: /deckhouse/modules/...
Error: template: node-manager/templates/node-group/node-group.yaml:5:12: executing ...
walk.go:74: found symbolic link in path: /deckhouse/modules/...
walk.go:74: found symbolic link in path: /deckhouse/modules/...
Error: template: node-manager/templates/node-group/node-group.yaml:5:12: executing ...`)

	var buf bytes.Buffer

	wrapper := FilteredHelmWriter{Writer: &buf}

	_, err := wrapper.Write(testData)
	require.NoError(t, err)

	result := `
Error: template: node-manager/templates/node-group/node-group.yaml:5:12: executing ...
Error: template: node-manager/templates/node-group/node-group.yaml:5:12: executing ...`
	require.Equal(t, buf.String(), result)
}
