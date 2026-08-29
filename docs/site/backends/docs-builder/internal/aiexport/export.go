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
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	corpusVersion = 1
	generator     = "hugo"
	// The names are qualified because the modules library shares its URL space
	// with the DKP documentation, which publishes an `llms.txt` and a
	// `corpus.json` of its own (see `_plugins/ai_export.rb` of the Jekyll site).
	corpusName = "external-corpus.json"
	llmsName   = "external-llms.txt"
	// An H2 section longer than this is split further, by H3.
	maxChunkChars = 6000
)

// Manifest is the build-time index produced by `layouts/_default/page.json`
// (`content/ai/ai.md`). Hugo knows the page metadata; this package knows how to
// turn the rendered HTML into Markdown, so the two meet here.
type Manifest struct {
	Version     int                `json:"version"`
	ProductCode string             `json:"productCode"`
	Generator   string             `json:"generator"`
	Lang        string             `json:"lang"`
	BaseURL     string             `json:"baseUrl"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Documents   []ManifestDocument `json:"documents"`
}

type ManifestDocument struct {
	Title string `json:"title"`
	// URL is the public path of the page, without the language prefix.
	URL string `json:"url"`
	// HTMLPath is the path of the rendered file relative to the public
	// directory, i.e. the `RelPermalink` including the language prefix.
	HTMLPath    string   `json:"htmlPath"`
	Module      string   `json:"module"`
	Channel     string   `json:"channel"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Keywords    []string `json:"keywords"`
	Tags        []string `json:"tags"`
	Editions    []string `json:"editions"`
	Stage       string   `json:"stage"`
	SearchBoost float64  `json:"searchBoost"`
}

// Corpus is the RAG-oriented export: page metadata, the full Markdown and the
// retrieval-sized chunks.
//
// The `desc` tags are the source of the JSON Schema embedded in `Schema`
// (schema.go); keep them accurate, they are the field documentation an agent
// reads out of the file itself.
type Corpus struct {
	Version     int             `json:"version" desc:"Corpus format version."`
	ProductCode string          `json:"productCode" desc:"Product the corpus belongs to."`
	Lang        string          `json:"lang" desc:"Language of the corpus (en or ru)."`
	Generator   string          `json:"generator" desc:"Generator that produced the corpus (hugo or jekyll)."`
	BaseURL     string          `json:"baseUrl" desc:"Site origin the relative URLs resolve against."`
	Schema      json.RawMessage `json:"schema" desc:"JSON Schema (draft 2020-12) of this corpus, embedded so the file describes itself."`
	Documents   []Document      `json:"documents" desc:"The exported pages."`
}

type Document struct {
	ID          string   `json:"id" desc:"Stable identifier of the page (SHA-1 of its URL)."`
	Title       string   `json:"title" desc:"Page title."`
	Description string   `json:"description" desc:"Short one-line description of the page."`
	URL         string   `json:"url" desc:"Public URL of the HTML page."`
	MdURL       string   `json:"mdUrl" desc:"Public URL of the Markdown twin of the page."`
	Path        string   `json:"path" desc:"URL path without language prefix or extension; a stable key."`
	Lang        string   `json:"lang" desc:"Language of the page (en or ru)."`
	Breadcrumbs []string `json:"breadcrumbs" desc:"Navigation trail from the site root to the page."`
	Keywords    []string `json:"keywords" desc:"Search keywords and tags."`
	Module      string   `json:"module,omitempty" desc:"Module the page belongs to, if any."`
	ModuleType  string   `json:"moduleType,omitempty" desc:"Kind of module: embedded or external."`
	Version     string   `json:"version,omitempty" desc:"Module or documentation version, if known."`
	Editions    []string `json:"editions" desc:"Editions the module is available in."`
	Stage       string   `json:"stage,omitempty" desc:"Module lifecycle stage, if any."`
	Channel     string   `json:"channel,omitempty" desc:"Release channel of the module, if any."`
	ContentHash string   `json:"contentHash" desc:"SHA-256 of the Markdown body."`
	Markdown    string   `json:"markdown" desc:"Full page body in Markdown."`
	Chunks      []Chunk  `json:"chunks" desc:"Retrieval-sized pieces of the page."`
}

type Chunk struct {
	ID           string   `json:"id" desc:"Identifier of the chunk (document id plus anchor or ordinal)."`
	Anchor       string   `json:"anchor,omitempty" desc:"HTML heading anchor the chunk starts at, if any."`
	URL          string   `json:"url" desc:"URL of the page, with the anchor fragment when present."`
	Level        int      `json:"level" desc:"Heading level of the chunk (1 for the preamble)."`
	HeadingPath  []string `json:"headingPath" desc:"Titles from the page title down to the chunk heading."`
	Markdown     string   `json:"markdown" desc:"Markdown of the chunk."`
	CharCount    int      `json:"charCount" desc:"Length of the chunk Markdown in characters."`
	ApproxTokens int      `json:"approxTokens" desc:"Rough token estimate (charCount / 4)."`
}

// Export converts every page listed in `<publicDir>/<lang>/ai/ai.json` to
// Markdown, writes the `.md` next to the rendered `.html` and publishes
// `<publicDir>/<lang>/modules/{external-llms.txt,external-corpus.json}`.
//
// A missing manifest is not an error: the site simply has no module pages yet.
func Export(publicDir, lang string, logger *log.Logger) error {
	if logger == nil {
		// The export is a best-effort step of the build (see `internal/docs`),
		// so a caller that forgot the logger should not take the build down.
		logger = log.NewNop()
	}

	manifestPath := filepath.Join(publicDir, lang, "ai", "ai.json")

	manifest, err := readManifest(manifestPath)
	if err != nil {
		return err
	}

	if manifest == nil || len(manifest.Documents) == 0 {
		logger.Debug("ai export skipped: no pages in the manifest",
			slog.String("lang", lang), slog.String("manifest", manifestPath))

		return nil
	}

	baseURL := strings.TrimRight(manifest.BaseURL, "/")
	documents := make([]Document, 0, len(manifest.Documents))

	// Directories that hold an authored index page (`index.html`); a module
	// readme in such a directory keeps its own name instead of being normalized
	// to `index.md` (see mdTwin).
	indexDirs := make(map[string]bool)
	for _, entry := range manifest.Documents {
		if path.Base(entry.URL) == "index.html" {
			indexDirs[path.Dir(entry.URL)] = true
		}
	}

	for _, entry := range manifest.Documents {
		document, err := exportPage(publicDir, baseURL, manifest, entry, indexDirs, logger)
		if err != nil {
			return err
		}

		if document != nil {
			documents = append(documents, *document)
		}
	}

	// Pages the manifest lists but Hugo has not rendered, and pages that
	// converted to nothing. A handful is normal; a jump means the manifest and
	// the rendered site have drifted apart.
	if skipped := len(manifest.Documents) - len(documents); skipped > 0 {
		logger.Info("ai export skipped pages",
			slog.String("lang", lang), slog.Int("count", skipped), slog.Int("total", len(manifest.Documents)))
	}

	if len(documents) == 0 {
		logger.Warn("ai export produced no documents", slog.String("lang", lang))

		return nil
	}

	slices.SortFunc(documents, func(a, b Document) int { return strings.Compare(a.Path, b.Path) })

	destDir := filepath.Join(publicDir, lang, "modules")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", destDir, err)
	}

	schema, err := corpusSchemaJSON()
	if err != nil {
		return fmt.Errorf("build corpus schema: %w", err)
	}

	corpus := Corpus{
		Version:     corpusVersion,
		ProductCode: manifest.ProductCode,
		Lang:        lang,
		Generator:   generator,
		BaseURL:     baseURL,
		Schema:      schema,
		Documents:   documents,
	}

	encoded, err := json.Marshal(corpus)
	if err != nil {
		return fmt.Errorf("encode corpus: %w", err)
	}

	if err := os.WriteFile(filepath.Join(destDir, corpusName), encoded, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", corpusName, err)
	}

	llms := renderLLMsTxt(manifest, baseURL, documents)
	if err := os.WriteFile(filepath.Join(destDir, llmsName), []byte(llms), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", llmsName, err)
	}

	logger.Info("ai export written",
		slog.String("lang", lang),
		slog.Int("documents", len(documents)),
		slog.Int("corpus_bytes", len(encoded)),
		slog.String("dir", destDir))

	return nil
}

func readManifest(manifestPath string) (*Manifest, error) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("read %s: %w", manifestPath, err)
	}

	manifest := new(Manifest)
	if err := json.Unmarshal(raw, manifest); err != nil {
		return nil, fmt.Errorf("parse %s: %w", manifestPath, err)
	}

	return manifest, nil
}

func exportPage(publicDir, baseURL string, manifest *Manifest, entry ManifestDocument, indexDirs map[string]bool, logger *log.Logger) (*Document, error) {
	htmlPath := filepath.Join(publicDir, filepath.FromSlash(strings.TrimPrefix(entry.HTMLPath, "/")))

	source, err := os.ReadFile(htmlPath)
	if err != nil {
		// Hugo may have skipped the page (a broken module is stripped and the
		// site rebuilt); the manifest is only a hint, not a contract.
		if errors.Is(err, fs.ErrNotExist) {
			logger.Debug("page is not rendered", slog.String("html_path", htmlPath))

			return nil, nil
		}

		return nil, fmt.Errorf("read %s: %w", htmlPath, err)
	}

	page, err := ConvertPage(string(source), Options{BaseURL: baseURL, PageURL: entry.URL, RewriteLink: linkRewriter(indexDirs)})
	if err != nil {
		return nil, fmt.Errorf("convert %s: %w", htmlPath, err)
	}

	if strings.TrimSpace(page.Markdown) == "" {
		logger.Debug("page converted to an empty document", slog.String("html_path", htmlPath))

		return nil, nil
	}

	title := page.Title
	if title == "" {
		title = entry.Title
	}

	if title == "" {
		// Pages with neither an `<h1>` nor a frontmatter title do exist —
		// `docs/internal/*` of a few modules. Dropping them would hide content
		// that the site itself publishes, but `- []( … )` in llms.txt is no
		// use to anyone either, so name them after their file.
		title = titleFromURL(entry.URL)

		logger.Debug("page has no title, named after its file",
			slog.String("url", entry.URL), slog.String("title", title))
	}

	description := collapseLine(entry.Description)
	editions := cleanList(entry.Editions)

	// The `.md` carries a frontmatter header; the corpus keeps the bare body
	// (and hashes it), because every frontmatter field is already a column of
	// the corpus document — duplicating them would only desync the two.
	front, err := renderFrontmatter(frontmatter{
		Title:       title,
		Description: description,
		Canonical:   baseURL + entry.URL,
		Lang:        manifest.Lang,
		Version:     entry.Version,
		Module:      entry.Module,
		ModuleType:  "external",
		Channel:     entry.Channel,
		Editions:    editions,
		Stage:       entry.Stage,
	})
	if err != nil {
		return nil, fmt.Errorf("frontmatter %s: %w", htmlPath, err)
	}

	// Both the file on disk and the published URL follow the same normalization:
	// the directory root (`readme.html`) becomes `index.md` unless the directory
	// has an authored index.
	twin := mdTwin(entry.URL, indexDirs)

	mdPath := filepath.Join(filepath.Dir(htmlPath), path.Base(twin))
	if err := os.WriteFile(mdPath, []byte(front+page.Markdown+"\n"), 0o644); err != nil {
		return nil, fmt.Errorf("write %s: %w", mdPath, err)
	}

	id := fmt.Sprintf("%x", sha1.Sum([]byte(entry.URL)))

	return &Document{
		ID:          id,
		Title:       title,
		Description: description,
		URL:         entry.URL,
		MdURL:       twin,
		Path:        strings.Trim(strings.TrimSuffix(entry.URL, ".html"), "/"),
		Lang:        manifest.Lang,
		Breadcrumbs: []string{manifest.Title, entry.Module, title},
		Keywords:    cleanList(append(slices.Clone(entry.Keywords), entry.Tags...)),
		Module:      entry.Module,
		ModuleType:  "external",
		Version:     entry.Version,
		Editions:    editions,
		Stage:       entry.Stage,
		Channel:     entry.Channel,
		ContentHash: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(page.Markdown))),
		Markdown:    page.Markdown,
		Chunks:      chunk(id, entry.URL, title, page.Markdown, page.Headings),
	}, nil
}

// frontmatter is the YAML header prepended to each per-page `.md`: enough to
// keep a chunk's provenance once the file is split for retrieval. The field
// names match the corpus document, so the two artifacts share one vocabulary.
type frontmatter struct {
	Title       string   `yaml:"title"`
	Description string   `yaml:"description,omitempty"`
	Canonical   string   `yaml:"canonical"`
	Lang        string   `yaml:"lang"`
	Version     string   `yaml:"version,omitempty"`
	Module      string   `yaml:"module,omitempty"`
	ModuleType  string   `yaml:"moduleType"`
	Channel     string   `yaml:"channel,omitempty"`
	Editions    []string `yaml:"editions,omitempty"`
	Stage       string   `yaml:"stage,omitempty"`
}

func renderFrontmatter(front frontmatter) (string, error) {
	body, err := yaml.Marshal(front)
	if err != nil {
		return "", err
	}

	return "---\n" + string(body) + "---\n\n", nil
}

// renderLLMsTxt builds the llms.txt-format index: one section per module, in the order
// the modules appear in the sorted document list.
func renderLLMsTxt(manifest *Manifest, baseURL string, documents []Document) string {
	var out strings.Builder

	out.WriteString("# " + manifest.Title + "\n\n")
	out.WriteString("> The content below is for Deckhouse Platform external modules.\n\n")
	out.WriteString("> Note that the documented version may differ from the version actually used in a cluster.\n\n")

	var order []string
	byModule := map[string][]Document{}
	for _, document := range documents {
		if _, seen := byModule[document.Module]; !seen {
			order = append(order, document.Module)
		}
		byModule[document.Module] = append(byModule[document.Module], document)
	}

	for _, module := range order {
		out.WriteString("## " + module + "\n")

		for _, document := range byModule[module] {
			out.WriteString("- [" + document.Title + "](" + baseURL + document.MdURL + ")")
			if document.Description != "" {
				out.WriteString(": " + document.Description)
			}
			out.WriteString("\n")
		}

		out.WriteString("\n")
	}

	return out.String()
}

// mdTwin is the public path of a page's Markdown twin. The directory root of a
// module (`readme.html`) is normalized to `index.md`, so a directory link
// resolves to it directly, without a server-side redirect. The exception is a
// directory that already holds an authored index (`index.html`): there the
// readme keeps its own `readme.md` name, so it does not clobber that index.
func mdTwin(pageURL string, indexDirs map[string]bool) string {
	if strings.HasSuffix(pageURL, "/") {
		return pageURL + "index.md"
	}

	base := path.Base(pageURL)

	name := strings.TrimSuffix(base, path.Ext(base)) + ".md"
	if base == "readme.html" && !indexDirs[path.Dir(pageURL)] {
		name = "index.md"
	}

	return path.Join(path.Dir(pageURL), name)
}

// indexMdPath matches the URL templates whose directory links are rewritten to
// `index.md` — the documentation and the modules library. Kept in sync with
// `INDEX_MD_PATH` of `_plugins/ai_export.rb`.
var indexMdPath = regexp.MustCompile(`^/(?:products/[^/]+/documentation|modules)/`)

// linkRewriter rewrites an internal documentation link to its Markdown twin:
// `/a/b.html` -> `/a/b.md`, and a directory `/a/b/` -> `/a/b/index.md`. A
// directory is rewritten only under the documentation and modules templates
// (see indexMdPath).
//
// The `.html` rewrite goes through mdTwin, so a link to a module's directory
// README — including an in-page anchor like `readme.html#section`, which is how
// a `#section` link on the page itself arrives here — follows the same
// `readme.html` -> `index.md` normalization as the file that is actually
// written. It is otherwise unconditional: a link to a page that has no `.md`
// (an HTML-only page, or a page from another build) is rewritten anyway, which
// keeps cross-build links working and matches the site's redirects.
func linkRewriter(indexDirs map[string]bool) func(string) string {
	return func(link string) string {
		head, tail := splitLinkTail(link)

		switch {
		case strings.HasSuffix(head, "/"):
			if !indexMdPath.MatchString(head) {
				// A directory outside the documentation and modules space keeps
				// its HTML URL.
				return link
			}

			head += "index.md"
		case strings.HasSuffix(head, ".html"):
			head = mdTwin(head, indexDirs)
		default:
			// Not a page link (an asset, or an extensionless path): leave it alone.
			return link
		}

		return head + tail
	}
}

// splitLinkTail separates the path from a trailing `?query` and/or `#fragment`
// so the path can be rewritten while the tail is preserved.
func splitLinkTail(link string) (string, string) {
	if i := strings.IndexAny(link, "?#"); i >= 0 {
		return link[:i], link[i:]
	}

	return link, ""
}

func collapseLine(text string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(text), " "))
}

// titleFromURL turns `/modules/upmeter/stable/internal/development.html` into
// "Development" — a last resort for untitled pages.
func titleFromURL(pageURL string) string {
	name := path.Base(strings.TrimSuffix(strings.TrimSuffix(pageURL, "/"), path.Ext(pageURL)))
	if name == "" || name == "." || name == "/" {
		return "Untitled"
	}

	words := strings.FieldsFunc(name, func(r rune) bool { return r == '_' || r == '-' })
	if len(words) == 0 {
		return "Untitled"
	}

	first := []rune(words[0])
	first[0] = unicode.ToUpper(first[0])
	words[0] = string(first)

	return strings.Join(words, " ")
}

func cleanList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" && !slices.Contains(cleaned, trimmed) {
			cleaned = append(cleaned, trimmed)
		}
	}

	return cleaned
}

// --- chunking ---------------------------------------------------------------

type headingMark struct {
	line  int
	level int
	text  string
	id    string
}

// chunk splits a page's Markdown into retrieval-sized pieces: one per H2
// section, with oversized sections split further by H3, plus the preamble
// before the first H2.
func chunk(docID, pageURL, title, markdown string, headings []Heading) []Chunk {
	lines := strings.Split(markdown, "\n")
	marks := scanHeadings(lines, headings)

	var sections []headingMark
	for _, mark := range marks {
		if mark.level == 2 {
			sections = append(sections, mark)
		}
	}

	chunks := make([]Chunk, 0, len(sections)+1)

	end := len(lines)
	if len(sections) > 0 {
		end = sections[0].line
	}
	if preamble := sliceLines(lines, 0, end); preamble != "" {
		chunks = append(chunks, newChunk(docID, pageURL, "", 1, []string{title}, preamble, len(chunks)))
	}

	for i, mark := range sections {
		stop := len(lines)
		if i+1 < len(sections) {
			stop = sections[i+1].line
		}

		body := sliceLines(lines, mark.line, stop)
		if body == "" {
			continue
		}

		var subsections []headingMark
		for _, candidate := range marks {
			if candidate.level == 3 && candidate.line > mark.line && candidate.line < stop {
				subsections = append(subsections, candidate)
			}
		}

		if len(body) <= maxChunkChars || len(subsections) == 0 {
			chunks = append(chunks, newChunk(docID, pageURL, mark.id, 2, []string{title, mark.text}, body, len(chunks)))

			continue
		}

		if head := sliceLines(lines, mark.line, subsections[0].line); head != "" {
			chunks = append(chunks, newChunk(docID, pageURL, mark.id, 2, []string{title, mark.text}, head, len(chunks)))
		}

		for j, sub := range subsections {
			subStop := stop
			if j+1 < len(subsections) {
				subStop = subsections[j+1].line
			}

			subBody := sliceLines(lines, sub.line, subStop)
			if subBody == "" {
				continue
			}

			chunks = append(chunks, newChunk(docID, pageURL, sub.id, 3, []string{title, mark.text, sub.text}, subBody, len(chunks)))
		}
	}

	return chunks
}

func sliceLines(lines []string, from, to int) string {
	return strings.TrimSpace(strings.Join(lines[from:to], "\n"))
}

func newChunk(docID, pageURL, anchor string, level int, headingPath []string, markdown string, ordinal int) Chunk {
	suffix := strconv.Itoa(ordinal)
	chunkURL := pageURL
	if anchor != "" {
		suffix = anchor
		chunkURL = pageURL + "#" + anchor
	}

	return Chunk{
		ID:           docID + "#" + suffix,
		Anchor:       anchor,
		URL:          chunkURL,
		Level:        level,
		HeadingPath:  headingPath,
		Markdown:     markdown,
		CharCount:    len(markdown),
		ApproxTokens: (len(markdown) + 3) / 4,
	}
}

var (
	headingLine   = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	headingAnchor = regexp.MustCompile(`\s*\{#[^}]*\}\s*$`)
)

// scanHeadings locates the heading lines of the Markdown and re-attaches the
// HTML anchors collected during conversion. Matching is sequential, so a `#`
// inside a fenced block cannot steal an anchor.
func scanHeadings(lines []string, headings []Heading) []headingMark {
	var (
		marks  []headingMark
		fence  string
		cursor int
	)

	for index, line := range lines {
		stripped := strings.TrimSpace(line)

		if fence != "" {
			if strings.HasPrefix(stripped, fence) {
				fence = ""
			}

			continue
		}

		if opening := fenceStart.FindString(stripped); opening != "" {
			fence = opening

			continue
		}

		match := headingLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		level := len(match[1])
		// heading() appends the anchor as ` {#id}`; strip it back off so the
		// text matches the Heading collected during conversion.
		text := strings.TrimSpace(headingAnchor.ReplaceAllString(match[2], ""))
		id := ""

		for scan := cursor; scan < len(headings); scan++ {
			if headings[scan].Level == level && headings[scan].Text == text {
				id = headings[scan].ID
				cursor = scan + 1

				break
			}
		}

		marks = append(marks, headingMark{line: index, level: level, text: text, id: id})
	}

	return marks
}
