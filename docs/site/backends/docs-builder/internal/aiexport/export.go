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
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
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
type Corpus struct {
	Version     int        `json:"version"`
	ProductCode string     `json:"productCode"`
	Lang        string     `json:"lang"`
	Generator   string     `json:"generator"`
	BaseURL     string     `json:"baseUrl"`
	Documents   []Document `json:"documents"`
}

type Document struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	URL         string   `json:"url"`
	MdURL       string   `json:"mdUrl"`
	Path        string   `json:"path"`
	Lang        string   `json:"lang"`
	Breadcrumbs []string `json:"breadcrumbs"`
	Keywords    []string `json:"keywords"`
	Module      string   `json:"module,omitempty"`
	ModuleType  string   `json:"moduleType"`
	Editions    []string `json:"editions"`
	Stage       string   `json:"stage"`
	Channel     string   `json:"channel,omitempty"`
	SearchBoost float64  `json:"searchBoost"`
	ContentHash string   `json:"contentHash"`
	Markdown    string   `json:"markdown"`
	Chunks      []Chunk  `json:"chunks"`
}

type Chunk struct {
	ID           string   `json:"id"`
	Anchor       string   `json:"anchor"`
	URL          string   `json:"url"`
	Level        int      `json:"level"`
	HeadingPath  []string `json:"headingPath"`
	Markdown     string   `json:"markdown"`
	CharCount    int      `json:"charCount"`
	ApproxTokens int      `json:"approxTokens"`
}

// Export converts every page listed in `<publicDir>/<lang>/ai/ai.json` to
// Markdown, writes the `.md` next to the rendered `.html` and publishes
// `<publicDir>/<lang>/modules/{external-llms.txt,external-corpus.json}`.
//
// A missing manifest is not an error: the site simply has no module pages yet.
func Export(publicDir, lang string) error {
	manifest, err := readManifest(filepath.Join(publicDir, lang, "ai", "ai.json"))
	if err != nil {
		return err
	}

	if manifest == nil || len(manifest.Documents) == 0 {
		return nil
	}

	baseURL := strings.TrimRight(manifest.BaseURL, "/")
	documents := make([]Document, 0, len(manifest.Documents))

	for _, entry := range manifest.Documents {
		document, err := exportPage(publicDir, baseURL, manifest, entry)
		if err != nil {
			return err
		}

		if document != nil {
			documents = append(documents, *document)
		}
	}

	if len(documents) == 0 {
		return nil
	}

	slices.SortFunc(documents, func(a, b Document) int { return strings.Compare(a.Path, b.Path) })

	destDir := filepath.Join(publicDir, lang, "modules")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", destDir, err)
	}

	corpus := Corpus{
		Version:     corpusVersion,
		ProductCode: manifest.ProductCode,
		Lang:        lang,
		Generator:   generator,
		BaseURL:     baseURL,
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

func exportPage(publicDir, baseURL string, manifest *Manifest, entry ManifestDocument) (*Document, error) {
	htmlPath := filepath.Join(publicDir, filepath.FromSlash(strings.TrimPrefix(entry.HTMLPath, "/")))

	source, err := os.ReadFile(htmlPath)
	if err != nil {
		// Hugo may have skipped the page (a broken module is stripped and the
		// site rebuilt); the manifest is only a hint, not a contract.
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("read %s: %w", htmlPath, err)
	}

	page, err := ConvertPage(string(source), Options{BaseURL: baseURL, PageURL: entry.URL})
	if err != nil {
		return nil, fmt.Errorf("convert %s: %w", htmlPath, err)
	}

	if strings.TrimSpace(page.Markdown) == "" {
		return nil, nil
	}

	mdPath := strings.TrimSuffix(htmlPath, filepath.Ext(htmlPath)) + ".md"
	if err := os.WriteFile(mdPath, []byte(page.Markdown+"\n"), 0o644); err != nil {
		return nil, fmt.Errorf("write %s: %w", mdPath, err)
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
	}

	id := fmt.Sprintf("%x", sha1.Sum([]byte(entry.URL)))

	return &Document{
		ID:          id,
		Title:       title,
		Description: collapseLine(entry.Description),
		URL:         entry.URL,
		MdURL:       mdURL(entry.URL),
		Path:        strings.Trim(strings.TrimSuffix(entry.URL, ".html"), "/"),
		Lang:        manifest.Lang,
		Breadcrumbs: []string{manifest.Title, entry.Module, title},
		Keywords:    cleanList(append(slices.Clone(entry.Keywords), entry.Tags...)),
		Module:      entry.Module,
		ModuleType:  "external",
		Editions:    cleanList(entry.Editions),
		Stage:       entry.Stage,
		Channel:     entry.Channel,
		SearchBoost: entry.SearchBoost,
		ContentHash: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(page.Markdown))),
		Markdown:    page.Markdown,
		Chunks:      chunk(id, entry.URL, title, page.Markdown, page.Headings),
	}, nil
}

// renderLLMsTxt builds the llms.txt-format index: one section per module, in the order
// the modules appear in the sorted document list.
func renderLLMsTxt(manifest *Manifest, baseURL string, documents []Document) string {
	var out strings.Builder

	out.WriteString("# " + manifest.Title + "\n\n")
	out.WriteString("> The content below is for Deckhouse Platform external modules.\n\n")
	out.WriteString("> Note, that described modules version can differ from the version actually used in a cluster.\n\n")

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

func mdURL(pageURL string) string {
	if strings.HasSuffix(pageURL, "/") {
		return pageURL + "index.md"
	}

	return strings.TrimSuffix(pageURL, path.Ext(pageURL)) + ".md"
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

var headingLine = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)

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
		text := strings.TrimSpace(match[2])
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
