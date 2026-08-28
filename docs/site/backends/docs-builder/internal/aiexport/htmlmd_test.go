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

package aiexport

import (
	"reflect"
	"strings"
	"testing"
)

func TestConvertFragment(t *testing.T) {
	tests := []struct {
		name string
		html string
		opts Options
		want string
	}{
		{
			name: "paragraph with inline markup",
			html: `<p>Hello <strong>world</strong> and <code>kubectl get po</code>.</p>`,
			want: "Hello **world** and `kubectl get po`.",
		},
		{
			// Whitespace hugging a `<code>` comes from the HTML layout, not from
			// the value, and inside the backticks it would become part of it.
			name: "code span is trimmed",
			html: `<p>Default: <code>false </code></p>`,
			want: "Default: `false`",
		},
		{
			name: "headings",
			html: `<h2 id="conf">Configuring</h2><p>Body.</p><h3 id="sub">Sub</h3>`,
			want: "## Configuring\n\nBody.\n\n### Sub",
		},
		{
			name: "alert",
			html: `<div class="warning alert__wrap"><svg class="alert__icon"><use xlink:href="#x"></use></svg>` +
				`<div><p>Be careful.</p><p>Really.</p></div></div>`,
			want: "> **WARNING:** Be careful.\n>\n> Really.",
		},
		{
			name: "details",
			html: `<div class="details"><p class="details__lnk">` +
				`<a href="javascript:void(0)" class="details__summary">More info</a></p>` +
				`<div class="details__content"><div class="expand"><p>Hidden text.</p></div></div></div>`,
			want: "#### More info\n\nHidden text.",
		},
		{
			name: "tabs render every panel",
			html: `<div class="tabs-block"><ul class="tabs tabs__container tabs__container--title">` +
				`<li class="tabs__item active">Linux</li><li class="tabs__item">macOS</li></ul>` +
				`<div id="a" class="tabs__container tabs__container--descr active"><p>Use apt.</p></div>` +
				`<div id="b" class="tabs__container tabs__container--descr"><p>Use brew.</p></div></div>`,
			want: "#### Linux\n\nUse apt.\n\n#### macOS\n\nUse brew.",
		},
		{
			name: "chroma line numbers table",
			html: `<div class="highlight"><div class="chroma"><table class="lntable"><tr>` +
				`<td class="lntd"><pre class="chroma"><code><span class="lnt">1</span></code></pre></td>` +
				`<td class="lntd"><pre class="chroma"><code class="language-yaml" data-lang="yaml">` +
				`<span class="nt">kind</span><span class="p">:</span> <span class="l">ModuleConfig</span>` +
				"\n</code></pre></td></tr></table></div></div>",
			want: "```yaml\nkind: ModuleConfig\n```",
		},
		{
			name: "plain pre falls back to plaintext",
			html: `<pre><code>echo hi</code></pre>`,
			want: "```plaintext\necho hi\n```",
		},
		{
			name: "table",
			html: `<table><thead><tr><th>Name</th><th>Type</th></tr></thead>` +
				`<tbody><tr><td>a</td><td><code>string</code></td></tr></tbody></table>`,
			want: "| Name | Type |\n| --- | --- |\n| a | `string` |",
		},
		{
			name: "nested list",
			html: `<ul><li>one<ul><li>two</li></ul></li><li>three</li></ul>`,
			want: "- one\n\n  - two\n- three",
		},
		{
			name: "ordered list",
			html: `<ol><li>first</li><li>second</li></ol>`,
			want: "1. first\n2. second",
		},
		{
			name: "noindex region is dropped",
			html: `<p>Keep.</p><!--noindex--><h2>Related</h2><ul><li>x</li></ul><!--/noindex--><p>Also keep.</p>`,
			want: "Keep.\n\nAlso keep.",
		},
		{
			name: "links are absolutized",
			html: `<p><a href="../cr.html">CR</a> and <a href="/modules/user-authn/">Module</a>` +
				` and <a href="https://example.com">Ext</a></p>`,
			opts: Options{BaseURL: "https://deckhouse.io", PageURL: "/modules/prompp/stable/configuration.html"},
			want: "[CR](https://deckhouse.io/modules/prompp/cr.html) and " +
				"[Module](https://deckhouse.io/modules/user-authn/) and [Ext](https://example.com)",
		},
		{
			name: "json schema parameter rows",
			html: `<ul class="resources"><li class="top-level-toggleable"><div class="resources__prop_wrap">` +
				`<div id="parameters-auth" class="resources__prop_name anchored">` +
				`<span class="plus-icon"><svg></svg></span>` +
				`<div><span class="ancestors">settings.</span><span>auth</span></div></div>` +
				`<span class="resources__prop_type">object</span>` +
				`<p class="resources__attrs required"><span class="resources__attrs_name required">Required.</span></p>` +
				`<div class="resources__prop_description"><p>Auth settings.</p></div>` +
				`<p class="resources__attrs"><span class="resources__attrs_name">Default:</span> ` +
				`<span class="resources__attrs_content"><code>{}</code></span></p>` +
				`</div><ul><li><div class="resources__prop_wrap">` +
				`<div class="resources__prop_name anchored">` +
				`<div><span class="ancestors">settings.auth.</span><span>password</span></div></div>` +
				`<span class="resources__prop_type">string</span>` +
				`<div class="resources__prop_description"><p>The password.</p></div></div></li></ul></li></ul>`,
			want: "- **settings.auth** (`object`, required)\n\n  Auth settings.\n\n  **Default:** `{}`\n\n" +
				"  - **settings.auth.password** (`string`)\n\n    The password.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ConvertFragment(test.html, test.opts)
			if err != nil {
				t.Fatalf("ConvertFragment: %v", err)
			}

			if got != test.want {
				t.Errorf("got:\n%s\n\nwant:\n%s", got, test.want)
			}
		})
	}
}

func TestConvertPage(t *testing.T) {
	source := `<html><body><div class="docs docs-modules">
  <div class="docs__wrap-title"><h1 class="docs__title">Prompp</h1></div>
  <!--noindex--><div class="breadcrumbs__container"><ol class="breadcrumbs"><li>x</li></ol></div><!--/noindex-->
  <div class="post-content">
    <div class="info alert__wrap"><div><p>Available in CE.</p></div></div>
    <h2 id="usage">Usage</h2>
    <p>Enable it.</p>
    <h3 id="notes">Notes</h3>
    <p>More.</p>
    <div class="tags"><b>Tags: </b>monitoring</div>
  </div>
  <div class="related-links"><a href="/x">Related</a></div>
</div></body></html>`

	page, err := ConvertPage(source, Options{})
	if err != nil {
		t.Fatalf("ConvertPage: %v", err)
	}

	// Everything outside `div.post-content` — breadcrumbs, `.tags`, the
	// related-links block — is left out.
	want := "# Prompp\n\n> **NOTE:** Available in CE.\n\n## Usage\n\nEnable it.\n\n### Notes\n\nMore."
	if page.Markdown != want {
		t.Errorf("markdown:\ngot:\n%s\n\nwant:\n%s", page.Markdown, want)
	}

	if page.Title != "Prompp" {
		t.Errorf("title = %q, want %q", page.Title, "Prompp")
	}

	wantHeadings := []Heading{
		{Level: 1, Text: "Prompp"},
		{Level: 2, Text: "Usage", ID: "usage"},
		{Level: 3, Text: "Notes", ID: "notes"},
	}
	if !reflect.DeepEqual(page.Headings, wantHeadings) {
		t.Errorf("headings = %#v, want %#v", page.Headings, wantHeadings)
	}
}

func TestChunk(t *testing.T) {
	markdown := strings.Join([]string{
		"# Platform configuration",
		"",
		"Intro text.",
		"",
		"## Configuring",
		"",
		"```yaml",
		"# not a heading",
		"kind: ModuleConfig",
		"```",
		"",
		"## Reference",
		"",
		"### First",
		"",
		strings.Repeat("x", 4000),
		"",
		"### Second",
		"",
		strings.Repeat("y", 4000),
	}, "\n")

	headings := []Heading{
		{Level: 1, Text: "Platform configuration"},
		{Level: 2, Text: "Configuring", ID: "configuring"},
		{Level: 2, Text: "Reference", ID: "reference"},
		{Level: 3, Text: "First", ID: "first"},
		{Level: 3, Text: "Second", ID: "second"},
	}

	chunks := chunk("abc123", "/modules/foo/stable/configuration.html", "Platform configuration", markdown, headings)

	wantAnchors := []string{"", "configuring", "reference", "first", "second"}
	wantLevels := []int{1, 2, 2, 3, 3}

	if len(chunks) != len(wantAnchors) {
		t.Fatalf("got %d chunks, want %d: %#v", len(chunks), len(wantAnchors), chunks)
	}

	for i, want := range wantAnchors {
		if chunks[i].Anchor != want {
			t.Errorf("chunk %d anchor = %q, want %q", i, chunks[i].Anchor, want)
		}
		if chunks[i].Level != wantLevels[i] {
			t.Errorf("chunk %d level = %d, want %d", i, chunks[i].Level, wantLevels[i])
		}
	}

	if !strings.Contains(chunks[1].Markdown, "# not a heading") {
		t.Error("the comment inside the fenced block was treated as a heading")
	}

	if chunks[0].URL != "/modules/foo/stable/configuration.html" {
		t.Errorf("preamble url = %q", chunks[0].URL)
	}

	if chunks[1].ID != "abc123#configuring" {
		t.Errorf("chunk id = %q", chunks[1].ID)
	}
}
