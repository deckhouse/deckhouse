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

package run

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// TestAStoreOverItsBudgetRefusesWrites is what lets a collection start on a store a fill can no longer
// write to: waiting for that fill would wait for the room only the collection gives back.
func TestAStoreOverItsBudgetRefusesWrites(t *testing.T) {
	dir := t.TempDir()
	blobs := filepath.Join(dir, "docker", "registry", "v2", "blobs", "sha256", "ab", "abcd")
	require.NoError(t, os.MkdirAll(blobs, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(blobs, "data"), make([]byte, 64<<10), 0o644))

	loop := &Loop{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), DataDir: dir}

	assert.False(t, loop.StoreRefusesWrites(), "nothing read yet is not a refusal")

	loop.desired.Store(&registryv1alpha1.RegistryStorageSpec{Store: registryv1alpha1.StorageStore{Size: "1Ki"}})
	assert.True(t, loop.StoreRefusesWrites(), "64 KiB in a 1 KiB budget")

	unmeasured := &Loop{Log: loop.Log}
	unmeasured.desired.Store(&registryv1alpha1.RegistryStorageSpec{Store: registryv1alpha1.StorageStore{Size: "1Ki"}})
	assert.False(t, unmeasured.StoreRefusesWrites(), "a store that cannot be measured is not a refusal")
}
