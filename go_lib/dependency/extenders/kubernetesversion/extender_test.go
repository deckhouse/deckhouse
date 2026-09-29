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

package kubernetesversion

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/dependency/versionmatcher"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// A failed watcher must not poison the extender: it is retried and picks up the version once it can watch the file.
func TestWatchForKubernetesVersionRetriesFailedWatcher(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kubectl_version")
	e := &Extender{logger: log.NewNop(), versionMatcher: versionmatcher.New(true)}
	e.versionMatcher.ChangeBaseVersion(semver.MustParse("1.30.0"))

	// the file is absent, so the first watchers fail on Add
	go e.watchForKubernetesVersion(path, 10*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	require.NoError(t, os.WriteFile(path, []byte("1.31.0"), 0o644))

	require.Eventually(t, func() bool {
		return e.versionMatcher.GetBaseVersion().Equal(semver.MustParse("1.31.0"))
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, e.err)
}
