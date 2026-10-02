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

package providerdir

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderDirLowercasesProvider(t *testing.T) {
	require.Equal(t, "/tmp/dl/dvp", ProviderDir("/tmp/dl", "DVP"))
	require.Equal(t, "/tmp/dl/yandex", ProviderDir("/tmp/dl", "yandex"))
}

func TestProviderDigestDir(t *testing.T) {
	require.Equal(t, "/tmp/dl/dvp@sha256:abc", ProviderDigestDir("/tmp/dl", "DVP", "sha256:abc"))
}

func TestValidatorPath(t *testing.T) {
	require.Equal(t, "/tmp/dl/dvp/validator", ValidatorPath("/tmp/dl", "Dvp"))
}

func TestDigestFromDir(t *testing.T) {
	require.Equal(t, "sha256:abc", DigestFromDir("/tmp/dl/dvp@sha256:abc"))
	require.Empty(t, DigestFromDir("/tmp/dl/dvp"))
}

func TestDeliveredFollowsTheAlias(t *testing.T) {
	root := t.TempDir()
	digestDir := ProviderDigestDir(root, "dvp", "sha256:abc")
	require.NoError(t, os.MkdirAll(digestDir, 0o755))
	require.NoError(t, os.Symlink(digestDir, ProviderDir(root, "dvp")))

	dir, digest, ok := Delivered(root, "DVP")
	require.True(t, ok)
	require.Equal(t, "sha256:abc", digest)
	require.Equal(t, "dvp@sha256:abc", filepath.Base(dir))
}

func TestDeliveredRejectsWhatIsNotACurrentBundle(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, root string)
	}{
		{
			name:  "no alias",
			setup: func(*testing.T, string) {},
		},
		{
			name: "plain directory instead of a symlink",
			setup: func(t *testing.T, root string) {
				require.NoError(t, os.MkdirAll(ProviderDir(root, "dvp"), 0o755))
			},
		},
		{
			name: "dangling symlink",
			setup: func(t *testing.T, root string) {
				require.NoError(t, os.Symlink(ProviderDigestDir(root, "dvp", "sha256:gone"), ProviderDir(root, "dvp")))
			},
		},
		{
			name: "target without a digest in its name",
			setup: func(t *testing.T, root string) {
				target := filepath.Join(root, "plain")
				require.NoError(t, os.MkdirAll(target, 0o755))
				require.NoError(t, os.Symlink(target, ProviderDir(root, "dvp")))
			},
		},
		{
			name: "target belongs to another provider",
			setup: func(t *testing.T, root string) {
				target := ProviderDigestDir(root, "yandex", "sha256:abc")
				require.NoError(t, os.MkdirAll(target, 0o755))
				require.NoError(t, os.Symlink(target, ProviderDir(root, "dvp")))
			},
		},
		{
			name: "target outside root",
			setup: func(t *testing.T, root string) {
				target := filepath.Join(t.TempDir(), "dvp@sha256:abc")
				require.NoError(t, os.MkdirAll(target, 0o755))
				require.NoError(t, os.Symlink(target, ProviderDir(root, "dvp")))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)

			_, _, ok := Delivered(root, "dvp")
			require.False(t, ok)
		})
	}
}
