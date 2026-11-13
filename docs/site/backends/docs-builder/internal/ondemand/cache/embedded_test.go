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

package cache

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEmbeddedVersions(t *testing.T) {
	history := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "embedded_modules_list.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	// Minor versions of 1 only, the newest first, each once: not latest,
	// not a patch, not a module listed twice under one version.
	e := NewEmbeddedVersions(history(t, `{"latest":["echo"],"v1.9":["echo","echo"],"v1.10":["echo","alb"],"v1.10.2":["echo"],"v2.0":["echo"]}`))
	if got, err := e.List("echo"); err != nil || !reflect.DeepEqual(got, []string{"v1.10", "v1.9"}) {
		t.Fatalf("echo = %v, %v", got, err)
	}
	if got, err := e.List("none"); err != nil || len(got) != 0 {
		t.Fatalf("a module never embedded = %v, %v", got, err)
	}

	if _, err := NewEmbeddedVersions(history(t, `{"v1.9":`)).List("echo"); err == nil {
		t.Fatal("no error with a corrupt history file")
	}
	absent := filepath.Join(t.TempDir(), "absent.json")
	late := NewEmbeddedVersions(absent)
	if _, err := late.List("echo"); err == nil {
		t.Fatal("no error without the history file")
	}
	// A failed read is tried again.
	if err := os.WriteFile(absent, []byte(`{"v1.9":["echo"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := late.List("echo"); err != nil || !reflect.DeepEqual(got, []string{"v1.9"}) {
		t.Fatalf("echo after the file appeared = %v, %v", got, err)
	}
}
