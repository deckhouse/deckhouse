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

package template

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
)

func TestPrepareBootstrapUsesDefaultClusterMasterEndpoints(t *testing.T) {
	dc := &directoryconfig.DirectoryConfig{
		DownloadDir:      "/tmp",
		DownloadCacheDir: "/tmp/cache",
	}

	metaConfig, err := config.ParseConfigFromData(context.TODO(), clusterConfig+initConfig, config.DummyPreparatorProvider(), dc)
	require.NoError(t, err)
	mingetPath := filepath.Join(t.TempDir(), "minget")
	require.NoError(t, os.WriteFile(mingetPath, []byte("test-minget"), 0o600))
	t.Setenv("DHCTL_MINGET_PATH", mingetPath)

	templateController := NewTemplateController("")
	defer templateController.Close()

	err = PrepareBootstrap(context.TODO(), templateController, "127.0.0.1", metaConfig, dc)
	require.NoError(t, err)

	renderedBootstrap, err := os.ReadFile(filepath.Join(templateController.TmpDir, "bootstrap", "01-bootstrap-prerequisites.sh"))
	require.NoError(t, err)

	content := string(renderedBootstrap)
	require.Contains(t, content, `echo -n '`+base64.StdEncoding.EncodeToString([]byte("test-minget"))+`' | base64 -d > "$path"`)
	require.NotContains(t, content, `local minget_b64=`)
	require.NotContains(t, content, `export MINGET_B64=`)
	require.Contains(t, content, `PACKAGES_PROXY_BOOTSTRAP_CLUSTER_UUID=""`)
	require.Contains(t, content, fmt.Sprintf(`export PACKAGES_PROXY_BOOTSTRAP_ADDRESSES="127.0.0.1:%d"`, testRPPBootstrapServerPort))
	require.NotContains(t, content, "PACKAGES_PROXY_KUBE_APISERVER_ENDPOINTS")
	require.Contains(t, content, `export PACKAGES_PROXY_ADDRESSES="127.0.0.1:5444"`)
}
