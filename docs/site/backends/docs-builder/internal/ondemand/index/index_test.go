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

package index

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMergeCorpusWithoutDocuments(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "corpus.json"), []byte(`{"version":1}`), 0600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := mergeCorpus([]string{dir}, &buf); err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Documents []json.RawMessage `json:"documents"`
	}
	if err := json.Unmarshal(buf.Bytes(), &corpus); err != nil || len(corpus.Documents) != 0 {
		t.Fatalf("corpus %q, err %v", buf.String(), err)
	}
}
