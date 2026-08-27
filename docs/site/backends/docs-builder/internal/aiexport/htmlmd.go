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

// Package aiexport turns the rendered module documentation into the
// AI-friendly artifacts: per-page Markdown, corpus.json and llms.txt.
//
// The Markdown is recovered from the rendered HTML rather than from the source
// files: the sources still carry Hugo shortcodes, and the CONFIGURATION/CR
// pages are generated from OpenAPI schemas by the `openapi/*` partials and
// exist as HTML only. The rendered HTML is also the only place where the
// heading anchors are available, which the chunker needs.
//
// This file mirrors docs/documentation/_plugins/html_to_markdown.rb, the Jekyll
// implementation used for the platform documentation and the embedded modules.
// Both sites render the same content wrapper (`h1.docs__title` +
// `div.post-content`), so the conversion rules are shared; keep the two in sync.
package aiexport

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Heading is a heading emitted into the Markdown, in document order. ID comes
// from the rendered HTML and is used as a chunk anchor.
type Heading struct {
	Level int
	Text  string
	ID    string
}

// Page is the result of converting a whole page.
type Page struct {
	Title    string
	Markdown string
	Headings []Heading
}

// Options configures link handling of the converter.
type Options struct {
	// BaseURL is the site origin (`https://deckhouse.io`) prepended to
	// root-relative links. Empty leaves links root-relative.
	BaseURL string
	// PageURL is the public path of the page being converted, used as the base
	// for relative links. It must be the public URL, not the `/en/…` build path.
	PageURL string
	// RewriteLink is applied to root-relative paths before BaseURL is prepended.
	RewriteLink func(string) string
}

// Elements that never contribute to the export.
var droppedTags = map[string]bool{
	"script": true, "style": true, "svg": true, "noscript": true,
	"iframe": true, "form": true, "button": true, "template": true,
	"link": true, "meta": true,
}

// Class names marking interactive chrome rather than documentation content.
var droppedClasses = map[string]bool{
	"tags": true, "githubEditButton": true, "anchorjs-link": true, "anchor": true,
	"plus-icon": true, "minus-icon": true,
	"breadcrumbs__container": true, "navigation": true, "feedback": true,
	"copy-code": true, "copy-button": true,
}

var droppedIDs = map[string]bool{"toc": true}

// `div.alert__wrap` level class => Markdown blockquote label.
var alertLabels = map[string]string{
	"info":    "NOTE",
	"warning": "WARNING",
	"danger":  "DANGER",
}

var alertLevels = []string{"info", "warning", "danger"}

var inlineTags = map[string]bool{
	"a": true, "abbr": true, "b": true, "br": true, "cite": true, "code": true,
	"del": true, "em": true, "i": true, "img": true, "ins": true, "kbd": true,
	"mark": true, "q": true, "s": true, "samp": true, "small": true, "span": true,
	"strong": true, "sub": true, "sup": true, "time": true, "tt": true, "u": true,
	"var": true, "wbr": true,
}

const defaultCodeLanguage = "plaintext"

var (
	collapseNewlines = regexp.MustCompile(`[ \t\r\f\v]*\n[ \t\r\f\v]*`)
	collapseSpaces   = regexp.MustCompile(`[ \t]{2,}`)
	innerNewlines    = regexp.MustCompile(`\s*\n\s*`)
	backtickRun      = regexp.MustCompile("`+")
	fenceStart       = regexp.MustCompile("^(`{3,}|~{3,})")
	absoluteScheme   = regexp.MustCompile(`(?i)^[a-z][a-z0-9+.\-]*:`)
)

// ConvertPage converts a full rendered page (`h1.docs__title` + `div.post-content`).
func ConvertPage(source string, opts Options) (Page, error) {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return Page{}, fmt.Errorf("parse html: %w", err)
	}

	c := newConverter(opts)

	var title string
	if node := findFirst(doc, matchTag("h1", "docs__title")); node != nil {
		title = strings.TrimSpace(collapse(c.inline(node)))
		if title != "" {
			c.headings = append(c.headings, Heading{Level: 1, Text: title, ID: attr(node, "id")})
		}
	}

	var body string
	if node := findFirst(doc, matchTag("div", "post-content")); node != nil {
		body = c.blocks(node)
	}

	parts := make([]string, 0, 2)
	if title != "" {
		parts = append(parts, "# "+title)
	}
	if strings.TrimSpace(body) != "" {
		parts = append(parts, body)
	}

	return Page{
		Title:    title,
		Markdown: normalize(strings.Join(parts, "\n\n")),
		Headings: c.headings,
	}, nil
}

// ConvertFragment converts an HTML fragment. Used by the tests and by callers
// that already extracted the content element.
func ConvertFragment(source string, opts Options) (string, error) {
	context := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}

	nodes, err := html.ParseFragment(strings.NewReader(source), context)
	if err != nil {
		return "", fmt.Errorf("parse fragment: %w", err)
	}

	for _, node := range nodes {
		context.AppendChild(node)
	}

	c := newConverter(opts)

	return normalize(c.blocks(context)), nil
}

type converter struct {
	opts     Options
	headings []Heading
	// Nodes excluded from the output of a single blocks() subtree, used to
	// re-render a JSON-schema parameter row without its header parts.
	skip map[*html.Node]bool
}

func newConverter(opts Options) *converter {
	opts.BaseURL = strings.TrimRight(opts.BaseURL, "/")

	return &converter{opts: opts, skip: map[*html.Node]bool{}}
}

// --- block level ------------------------------------------------------------

// blocks renders the children of node as a sequence of Markdown blocks joined
// by a blank line. Consecutive inline children are merged into one paragraph;
// `<!--noindex-->…<!--/noindex-->` regions are skipped.
func (c *converter) blocks(node *html.Node) string {
	var (
		out      []string
		pending  strings.Builder
		skipping bool
	)

	flush := func() {
		text := strings.TrimSpace(collapse(pending.String()))
		if text != "" {
			out = append(out, text)
		}
		pending.Reset()
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case html.CommentNode:
			switch strings.TrimSpace(child.Data) {
			case "noindex":
				skipping = true
			case "/noindex":
				skipping = false
			}

			continue
		case html.TextNode:
			if !skipping {
				pending.WriteString(child.Data)
			}

			continue
		case html.ElementNode:
			if skipping || c.dropped(child) {
				continue
			}
		default:
			continue
		}

		if isInline(child) {
			pending.WriteString(c.inlineFor(child))

			continue
		}

		flush()

		if block := c.blockFor(child); strings.TrimSpace(block) != "" {
			out = append(out, block)
		}
	}

	flush()

	return strings.Join(out, "\n\n")
}

func (c *converter) blockFor(el *html.Node) string {
	switch el.Data {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return c.heading(el)
	case "p":
		return c.paragraph(el)
	case "pre":
		return c.codeBlock(el, "")
	case "ul", "ol":
		return c.list(el)
	case "dl":
		return c.definitionList(el)
	case "table":
		return c.table(el)
	case "blockquote":
		return c.blockquote(el)
	case "hr":
		return "---"
	case "details":
		return c.nativeDetails(el)
	}

	if block, ok := c.specialBlock(el); ok {
		return block
	}

	return c.blocks(el)
}

// specialBlock handles the Deckhouse-specific block constructs. The second
// result is false when el is a plain wrapper that should just be recursed into.
func (c *converter) specialBlock(el *html.Node) (string, bool) {
	switch {
	case hasClass(el, "alert__wrap"):
		return c.alert(el), true
	case hasClass(el, "details") && findFirst(el, matchClass("details__summary")) != nil:
		return c.details(el), true
	case hasClass(el, "tabs-block"):
		return c.tabs(el), true
	case hasClass(el, "highlight") || hasClass(el, "highlighter-rouge"):
		return c.highlight(el), true
	}

	return "", false
}

func (c *converter) heading(el *html.Node) string {
	level, err := strconv.Atoi(el.Data[1:])
	if err != nil {
		return ""
	}

	text := strings.TrimSpace(innerNewlines.ReplaceAllString(collapse(c.inline(el)), " "))
	if text == "" {
		return ""
	}

	c.headings = append(c.headings, Heading{Level: level, Text: text, ID: attr(el, "id")})

	return strings.Repeat("#", level) + " " + text
}

// paragraph renders a `<p>`. It normally holds inline content only, but the
// OpenAPI partials occasionally emit block markup inside one.
func (c *converter) paragraph(el *html.Node) string {
	for child := el.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && !isInline(child) && !c.dropped(child) {
			return c.blocks(el)
		}
	}

	return strings.TrimSpace(collapse(c.inline(el)))
}

func (c *converter) alert(el *html.Node) string {
	level := "info"
	for _, candidate := range alertLevels {
		if hasClass(el, candidate) {
			level = candidate

			break
		}
	}

	body := strings.TrimSpace(c.blocks(el))
	if body == "" {
		return ""
	}

	lines := strings.Split(body, "\n")
	lines[0] = "**" + alertLabels[level] + ":** " + lines[0]

	return quote(lines)
}

func (c *converter) details(el *html.Node) string {
	summary := "Details"
	if node := findFirst(el, matchClass("details__summary")); node != nil {
		if text := strings.TrimSpace(collapse(textContent(node))); text != "" {
			summary = text
		}
	}

	content := findFirst(el, matchClass("details__content"))
	if content == nil {
		content = el
	}

	return joinNonEmpty("\n\n", "#### "+summary, strings.TrimSpace(c.blocks(content)))
}

func (c *converter) nativeDetails(el *html.Node) string {
	summary := "Details"

	summaryNode := findFirst(el, matchTag("summary", ""))
	if summaryNode != nil {
		if text := strings.TrimSpace(collapse(c.inline(summaryNode))); text != "" {
			summary = text
		}

		c.skip[summaryNode] = true
		defer delete(c.skip, summaryNode)
	}

	return joinNonEmpty("\n\n", "#### "+summary, strings.TrimSpace(c.blocks(el)))
}

// tabs renders `div.tabs-block`: a list of tab labels followed by one panel per
// tab. All tabs are included in the export — an agent has no way to click.
func (c *converter) tabs(el *html.Node) string {
	var labels []string
	for _, list := range childElements(el, matchTag("ul", "")) {
		for _, item := range childElements(list, matchTag("li", "")) {
			labels = append(labels, strings.TrimSpace(collapse(textContent(item))))
		}
	}

	panels := childElements(el, matchTag("div", "tabs__container--descr"))
	if len(panels) == 0 {
		for _, panel := range childElements(el, matchTag("div", "tabs__container")) {
			if !hasClass(panel, "tabs__container--title") {
				panels = append(panels, panel)
			}
		}
	}

	sections := make([]string, 0, len(panels))
	for i, panel := range panels {
		label := ""
		if i < len(labels) {
			label = labels[i]
		}
		if label == "" {
			label = fmt.Sprintf("Tab %d", i+1)
		}

		sections = append(sections, joinNonEmpty("\n\n", "#### "+label, strings.TrimSpace(c.blocks(panel))))
	}

	return joinNonEmpty("\n\n", sections...)
}

// highlight unwraps the Chroma (`div.highlight > table.lntable`) and Rouge
// (`div.language-yaml.highlighter-rouge`) wrappers around a code block.
func (c *converter) highlight(el *html.Node) string {
	pre := lastCodeCellPre(el)
	if pre == nil {
		pre = findFirst(el, matchTag("pre", ""))
	}
	if pre == nil {
		return c.blocks(el)
	}

	language := detectLanguage(el)
	if language == "" {
		language = detectLanguage(pre)
	}

	return c.codeBlock(pre, language)
}

func (c *converter) codeBlock(pre *html.Node, language string) string {
	code := findFirst(pre, matchTag("code", ""))
	if code == nil {
		code = pre
	}

	text := strings.TrimRight(textContent(code), "\n")
	if strings.TrimSpace(text) == "" {
		return ""
	}

	if language == "" {
		language = detectLanguage(pre)
	}
	if language == "" {
		language = defaultCodeLanguage
	}

	fence := strings.Repeat("`", max(3, longestBacktickRun(text)+1))

	return fence + language + "\n" + text + "\n" + fence
}

// detectLanguage looks for a `language-*` class or a `data-lang` attribute on
// the node itself, up to three ancestors, and finally on the inner `<code>`.
func detectLanguage(node *html.Node) string {
	for current, depth := node, 0; current != nil && current.Type == html.ElementNode && depth < 4; current, depth = current.Parent, depth+1 {
		for _, class := range classList(current) {
			if lang, ok := strings.CutPrefix(class, "language-"); ok {
				return lang
			}
		}

		if lang := attr(current, "data-lang"); lang != "" {
			return lang
		}
	}

	code := findFirst(node, matchTag("code", ""))
	if code == nil {
		return ""
	}

	for _, class := range classList(code) {
		if lang, ok := strings.CutPrefix(class, "language-"); ok {
			return lang
		}
	}

	return ""
}

func (c *converter) list(el *html.Node) string {
	items := childElements(el, matchTag("li", ""))
	if len(items) == 0 {
		return ""
	}

	start := 1
	if value := attr(el, "start"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			start = parsed
		}
	}

	lines := make([]string, 0, len(items))
	for i, item := range items {
		// Nested `ul`s of a JSON-schema reference carry no `.resources` class,
		// so the parameter rows are detected per list item.
		var body string
		if len(childElements(item, matchTag("div", "resources__prop_wrap"))) > 0 {
			body = c.resourceItem(item)
		} else {
			body = c.blocks(item)
		}

		marker := "- "
		if el.Data == "ol" {
			marker = strconv.Itoa(start+i) + ". "
		}

		if entry := indent(body, marker); strings.TrimSpace(entry) != "" {
			lines = append(lines, entry)
		}
	}

	return strings.Join(lines, "\n")
}

// resourceItem renders a parameter row of a JSON-schema reference page
// (`ul.resources`), as produced by the `openapi/*` partials.
func (c *converter) resourceItem(li *html.Node) string {
	wraps := childElements(li, matchTag("div", "resources__prop_wrap"))
	if len(wraps) == 0 {
		return c.blocks(li)
	}
	wrap := wraps[0]

	nameNode := findFirst(wrap, matchClass("resources__prop_name"))
	name := resourceName(nameNode)

	var attributes []string
	typeNodes := childElements(wrap, matchTag("span", "resources__prop_type"))
	if len(typeNodes) > 0 {
		if typeName := strings.TrimSpace(textContent(typeNodes[0])); typeName != "" {
			attributes = append(attributes, "`"+typeName+"`")
		}
	}
	if findFirst(wrap, matchTag("span", "resources__attrs_name", "required")) != nil {
		attributes = append(attributes, "required")
	}
	if findFirst(wrap, matchClass("resources__prop_is_deprecated")) != nil {
		attributes = append(attributes, "deprecated")
	}

	header := ""
	if name != "" {
		header = "**" + name + "**"
	}
	if len(attributes) > 0 {
		header += " (" + strings.Join(attributes, ", ") + ")"
	}

	// The header parts are rendered above; hide them from the description pass.
	hidden := findAll(wrap, matchClass("resources__prop_name"))
	hidden = append(hidden, typeNodes...)
	hidden = append(hidden, findAll(wrap, matchTag("p", "resources__attrs", "required"))...)
	for _, node := range hidden {
		c.skip[node] = true
	}

	description := strings.TrimSpace(c.blocks(wrap))

	for _, node := range hidden {
		delete(c.skip, node)
	}

	parts := []string{strings.TrimSpace(header), description}
	for _, nested := range childElements(li, matchTag("ul", "")) {
		parts = append(parts, c.list(nested))
	}

	return joinNonEmpty("\n\n", parts...)
}

func resourceName(nameNode *html.Node) string {
	if nameNode == nil {
		return ""
	}

	ancestors := ""
	if node := findFirst(nameNode, matchTag("span", "ancestors")); node != nil {
		ancestors = strings.TrimSpace(textContent(node))
	}

	own := ""
	if holder := findFirst(nameNode, matchTag("div", "")); holder != nil {
		for _, span := range childElements(holder, matchTag("span", "")) {
			if !hasClass(span, "ancestors") {
				own += textContent(span)
			}
		}
	} else {
		own = textContent(nameNode)
	}

	return strings.TrimSpace(ancestors + strings.TrimSpace(collapse(own)))
}

func (c *converter) definitionList(el *html.Node) string {
	type entry struct {
		term        string
		definitions []string
	}

	var items []entry
	for child := el.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}

		switch child.Data {
		case "dt":
			items = append(items, entry{term: strings.TrimSpace(collapse(c.inline(child)))})
		case "dd":
			if len(items) == 0 {
				items = append(items, entry{})
			}
			last := &items[len(items)-1]
			last.definitions = append(last.definitions, strings.TrimSpace(c.blocks(child)))
		}
	}

	lines := make([]string, 0, len(items))
	for _, item := range items {
		parts := make([]string, 0, len(item.definitions)+1)
		if item.term != "" {
			parts = append(parts, "**"+item.term+"**")
		}
		parts = append(parts, item.definitions...)

		if body := indent(joinNonEmpty("\n\n", parts...), "- "); strings.TrimSpace(body) != "" {
			lines = append(lines, body)
		}
	}

	return strings.Join(lines, "\n")
}

func (c *converter) table(el *html.Node) string {
	// Chroma renders line numbers as a two-column table; only the code matters.
	if hasClass(el, "lntable") {
		return c.highlight(el)
	}

	var rows [][]string
	for _, tr := range findAll(el, matchTag("tr", "")) {
		var row []string
		for cell := tr.FirstChild; cell != nil; cell = cell.NextSibling {
			if cell.Type == html.ElementNode && (cell.Data == "th" || cell.Data == "td") {
				row = append(row, c.tableCell(cell))
			}
		}

		if len(row) > 0 {
			rows = append(rows, row)
		}
	}

	if len(rows) == 0 {
		return ""
	}

	width := 0
	for _, row := range rows {
		width = max(width, len(row))
	}
	for i, row := range rows {
		for len(row) < width {
			row = append(row, "")
		}
		rows[i] = row
	}

	separators := make([]string, width)
	for i := range separators {
		separators[i] = "---"
	}

	lines := []string{
		"| " + strings.Join(rows[0], " | ") + " |",
		"| " + strings.Join(separators, " | ") + " |",
	}
	for _, row := range rows[1:] {
		lines = append(lines, "| "+strings.Join(row, " | ")+" |")
	}

	return strings.Join(lines, "\n")
}

func (c *converter) tableCell(cell *html.Node) string {
	text := strings.TrimSpace(innerNewlines.ReplaceAllString(collapse(c.inline(cell)), " "))

	return strings.ReplaceAll(text, "|", `\|`)
}

func (c *converter) blockquote(el *html.Node) string {
	body := strings.TrimSpace(c.blocks(el))
	if body == "" {
		return ""
	}

	return quote(strings.Split(body, "\n"))
}

// --- inline level -----------------------------------------------------------

func (c *converter) inline(node *html.Node) string {
	var (
		buffer   strings.Builder
		skipping bool
	)

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case html.CommentNode:
			switch strings.TrimSpace(child.Data) {
			case "noindex":
				skipping = true
			case "/noindex":
				skipping = false
			}
		case html.TextNode:
			if !skipping {
				buffer.WriteString(child.Data)
			}
		case html.ElementNode:
			if !skipping && !c.dropped(child) {
				buffer.WriteString(c.inlineFor(child))
			}
		}
	}

	return buffer.String()
}

func (c *converter) inlineFor(el *html.Node) string {
	switch el.Data {
	case "br":
		return "\n"
	case "code", "kbd", "samp", "tt":
		return codeSpan(el)
	case "strong", "b":
		return emphasize(c.inline(el), "**")
	case "em", "i", "var", "cite":
		return emphasize(c.inline(el), "*")
	case "del", "s":
		return emphasize(c.inline(el), "~~")
	case "a":
		return c.link(el)
	case "img":
		return c.image(el)
	case "wbr":
		return ""
	}

	text := c.inline(el)
	// Attribute captions of JSON-schema pages ("Default:", "Pattern:").
	if hasClass(el, "resources__attrs_name") {
		return emphasize(text, "**")
	}

	return text
}

func codeSpan(el *html.Node) string {
	text := strings.ReplaceAll(strings.ReplaceAll(textContent(el), "\r\n", " "), "\n", " ")
	if strings.TrimSpace(text) == "" {
		return ""
	}

	ticks := strings.Repeat("`", longestBacktickRun(text)+1)

	pad := ""
	if strings.HasPrefix(text, "`") || strings.HasSuffix(text, "`") {
		pad = " "
	}

	return ticks + pad + text + pad + ticks
}

func emphasize(text, marker string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return text
	}

	lead := text[:strings.Index(text, trimmed)]
	tail := text[len(lead)+len(trimmed):]

	return lead + marker + trimmed + marker + tail
}

func (c *converter) link(el *html.Node) string {
	href := strings.TrimSpace(attr(el, "href"))
	text := strings.TrimSpace(collapse(c.inline(el)))

	if href == "" || strings.HasPrefix(href, "javascript:") {
		return text
	}

	target := c.absolutize(href)
	if text == "" {
		text = target
	}

	return "[" + text + "](" + target + ")"
}

func (c *converter) image(el *html.Node) string {
	src := strings.TrimSpace(attr(el, "src"))
	if src == "" {
		return ""
	}

	return "![" + strings.TrimSpace(attr(el, "alt")) + "](" + c.absolutize(src) + ")"
}

func (c *converter) absolutize(href string) string {
	if absoluteScheme.MatchString(href) || strings.HasPrefix(href, "//") {
		return href
	}

	var path string

	switch {
	case strings.HasPrefix(href, "#"):
		path = c.opts.PageURL + href
	case strings.HasPrefix(href, "/"):
		path = href
	default:
		base, err := url.Parse("https://example.invalid" + c.opts.PageURL)
		if err != nil {
			return href
		}

		ref, err := url.Parse(href)
		if err != nil {
			return href
		}

		resolved := base.ResolveReference(ref)
		path = resolved.EscapedPath()
		if resolved.RawQuery != "" {
			path += "?" + resolved.RawQuery
		}
		if resolved.Fragment != "" {
			path += "#" + resolved.EscapedFragment()
		}
	}

	if c.opts.RewriteLink != nil {
		path = c.opts.RewriteLink(path)
	}

	return c.opts.BaseURL + path
}

// --- helpers ----------------------------------------------------------------

func (c *converter) dropped(el *html.Node) bool {
	if c.skip[el] || droppedTags[el.Data] || droppedIDs[attr(el, "id")] {
		return true
	}

	for _, class := range classList(el) {
		if droppedClasses[class] {
			return true
		}
	}

	return false
}

func isInline(el *html.Node) bool {
	return inlineTags[el.Data]
}

func attr(node *html.Node, name string) string {
	for _, a := range node.Attr {
		if a.Key == name {
			return a.Val
		}
	}

	return ""
}

func classList(node *html.Node) []string {
	return strings.Fields(attr(node, "class"))
}

func hasClass(node *html.Node, class string) bool {
	for _, candidate := range classList(node) {
		if candidate == class {
			return true
		}
	}

	return false
}

type matcher func(*html.Node) bool

// matchTag matches an element by tag name (empty matches any) and by every
// listed class.
func matchTag(tag string, classes ...string) matcher {
	return func(node *html.Node) bool {
		if tag != "" && node.Data != tag {
			return false
		}

		for _, class := range classes {
			if class != "" && !hasClass(node, class) {
				return false
			}
		}

		return true
	}
}

func matchClass(class string) matcher {
	return matchTag("", class)
}

func findFirst(node *html.Node, match matcher) *html.Node {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}

		if match(child) {
			return child
		}

		if found := findFirst(child, match); found != nil {
			return found
		}
	}

	return nil
}

func findAll(node *html.Node, match matcher) []*html.Node {
	var found []*html.Node

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}

		if match(child) {
			found = append(found, child)
		}

		found = append(found, findAll(child, match)...)
	}

	return found
}

func childElements(node *html.Node, match matcher) []*html.Node {
	var found []*html.Node

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && match(child) {
			found = append(found, child)
		}
	}

	return found
}

// lastCodeCellPre returns the `pre` of the code column of a Chroma line-number
// table (`td.lntd:last-child pre`).
func lastCodeCellPre(node *html.Node) *html.Node {
	cells := findAll(node, matchTag("td", "lntd"))
	if len(cells) == 0 {
		return nil
	}

	return findFirst(cells[len(cells)-1], matchTag("pre", ""))
}

func textContent(node *html.Node) string {
	var buffer strings.Builder

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			buffer.WriteString(n.Data)

			return
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)

	return buffer.String()
}

func collapse(text string) string {
	return collapseSpaces.ReplaceAllString(collapseNewlines.ReplaceAllString(text, "\n"), " ")
}

func longestBacktickRun(text string) int {
	longest := 0
	for _, run := range backtickRun.FindAllString(text, -1) {
		longest = max(longest, len(run))
	}

	return longest
}

func quote(lines []string) string {
	quoted := make([]string, len(lines))
	for i, line := range lines {
		if line == "" {
			quoted[i] = ">"
		} else {
			quoted[i] = "> " + line
		}
	}

	return strings.Join(quoted, "\n")
}

func joinNonEmpty(separator string, parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			kept = append(kept, part)
		}
	}

	return strings.Join(kept, separator)
}

// indent prefixes the first line with marker and aligns the rest under it, so
// that nested blocks stay inside the list item.
func indent(text, marker string) string {
	pad := strings.Repeat(" ", len(marker))
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		switch {
		case i == 0:
			lines[i] = strings.TrimRight(marker+line, " \t")
		case line == "":
		default:
			lines[i] = pad + line
		}
	}

	return strings.Join(lines, "\n")
}

// normalize trims trailing whitespace and collapses runs of blank lines,
// leaving fenced code blocks untouched.
func normalize(text string) string {
	var (
		lines []string
		fence string
		blank int
	)

	for _, line := range strings.Split(text, "\n") {
		if fence != "" {
			lines = append(lines, line)
			if strings.HasPrefix(strings.TrimSpace(line), fence) {
				fence = ""
			}

			continue
		}

		if opening := fenceStart.FindString(strings.TrimSpace(line)); opening != "" {
			fence = opening
			blank = 0
			lines = append(lines, strings.TrimRight(line, " \t"))

			continue
		}

		stripped := strings.TrimRight(line, " \t")
		if stripped == "" {
			blank++
			if blank <= 1 {
				lines = append(lines, "")
			}

			continue
		}

		blank = 0
		lines = append(lines, stripped)
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}
