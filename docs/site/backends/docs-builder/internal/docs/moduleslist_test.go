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
	"context"
	"strings"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestRenderModulesList(t *testing.T) {
	data := `{"modules":[
		{"module":"echo","title":{"en":"Echo"},"description":{"en":"Answers what it hears."},"stage":"Preview","enabled":true},
		{"module":"pending","pending":true},
		{"module":"bare","noDocs":true}
	]}`

	pages, err := RenderModulesList(context.Background(), "../../../docs-builder-template", t.TempDir(), []byte(data), log.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if len(pages["ru"]) == 0 {
		t.Error("no russian page")
	}

	en := string(pages["en"])
	for _, want := range []string{
		`href="echo/"`,
		"Answers what it hears.",
		// Not rendered yet: shown by its name, with a link to the module and
		// a note that the details come later.
		`href="pending/"`,
		"The documentation of the module is loading, the description will appear later",
		// No documentation: a tile without a link, marked so.
		"No documentation in the published versions",
		// The page takes the descriptions of the pending cards from the
		// rewritten list by the module of the tile.
		`data-module="pending"`,
		`data-module="bare"`,
		"querySelectorAll(pendingSelector)",
	} {
		if !strings.Contains(en, want) {
			t.Errorf("english page lacks %q", want)
		}
	}
	if strings.Contains(en, `href="bare/"`) {
		t.Error("a module without documentation is linked")
	}
}
