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

package upstream

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// The main list is seeded from the fallback list right after Start, before the
// proxy port opens, so Start must have read the file by the time it returns.
func TestFallbackList_StartLoadsFileBeforeReturning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upstreams.json")
	if err := os.WriteFile(path, []byte(`["10.0.0.1:6443","10.0.0.2:6443"]`), 0o600); err != nil {
		t.Fatal(err)
	}

	fb, err := NewFallbackList(WithFallbackLogger(log.NewNop()), WithFileWatcher(path))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	if err := fb.Start(ctx); err != nil {
		t.Fatal(err)
	}

	if got, want := fb.ListFullAddresses(), []string{"10.0.0.1:6443", "10.0.0.2:6443"}; !slices.Equal(got, want) {
		t.Fatalf("addresses right after Start = %v, want %v", got, want)
	}
}
