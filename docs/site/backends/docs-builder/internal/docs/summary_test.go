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

package docs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestWriteSummary(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  Summary
	}{
		{
			name: "menuTitle and descriptions of module.yaml win",
			files: map[string]string{
				"data/modules/echo/v1.0.0/module.yaml":     "stage: Preview\ndescriptions:\n  en: Answers.\n  ru: Answers in Russian.\n",
				"content/modules/echo/v1.0.0/README.md":    "---\ntitle: The echo module\nlinkTitle: Echo link\nmenuTitle: Echo\ndescription: From README.\n---\nBody\n",
				"content/modules/echo/v1.0.0/README.ru.md": "\xef\xbb\xbf---\ntitle: Echo ru\n---\nBody\n",
			},
			want: Summary{
				Title:       map[string]string{"en": "Echo", "ru": "Echo ru"},
				Description: map[string]string{"en": "Answers.", "ru": "Answers in Russian."},
				Stage:       "Preview",
			},
		},
		{
			name: "README only",
			files: map[string]string{
				"content/modules/echo/v1.0.0/README.md": "---\nlinkTitle: Echo\ndescription: From README.\n---\n",
			},
			want: Summary{
				Title:       map[string]string{"en": "Echo"},
				Description: map[string]string{"en": "From README."},
			},
		},
		{
			name: "no front matter, broken module.yaml",
			files: map[string]string{
				"data/modules/echo/v1.0.0/module.yaml":  "stage: [",
				"content/modules/echo/v1.0.0/README.md": "# Echo\n",
			},
			want: Summary{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			site, out := t.TempDir(), t.TempDir()
			for name, body := range tt.files {
				path := filepath.Join(site, name)
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}

			if err := writeSummary(site, out, "echo", "v1.0.0", log.NewNop()); err != nil {
				t.Fatal(err)
			}
			got, err := ReadSummary(out)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("summary = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestReadSummaryMissing(t *testing.T) {
	if _, err := ReadSummary(t.TempDir()); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want fs.ErrNotExist", err)
	}
}
