#!/usr/bin/env python3
# Copyright 2026 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Link glossary terms in documentation pages.

For every page under docs/documentation/pages the script finds the first
mention of each glossary term in every second-level section (the text before
the first H2 counts as a section) and turns it into a link to the glossary
page: ``[term](../glossary.html#<id>)``. Later mentions in the same section
stay as they are. Headings, code, existing links, Liquid tags, HTML tags and
front matter are never touched. A term that is already linked in a section
is not linked again, so the script can be re-run after new pages are added.

Patterns come from term-patterns.yaml next to this script. Without --apply
the script only prints what it would change.
"""

import argparse
import bisect
import collections
import pathlib
import re
import sys

import yaml

HERE = pathlib.Path(__file__).resolve().parent
DEFAULT_PATTERNS = HERE / "term-patterns.yaml"
DEFAULT_PAGES = "docs/documentation/pages"
DEFAULT_GLOSSARY = "docs/documentation/_glossary"
EXCLUDED_DIRS = ("drafts", "internal")
EXCLUDED_FILES = ("GLOSSARY.md", "GLOSSARY_RU.md", "404.md", "404_RU.md")
MAX_LINE_LENGTH = 1000  # MD013 limit in testing/markdownlint.yaml; a link is not added to a line it would push over

FRONT_MATTER_RE = re.compile(r"\A---\n(.*?)\n---\n", re.S)
PROTECTED_RES = [
    re.compile(r"^[ \t]*(```|~~~)[^\n]*\n.*?^[ \t]*\1[ \t]*$", re.S | re.M),  # fenced code, also indented in lists
    re.compile(r"«[^»\n]{1,80}»"),  # quoted names of UI elements and sections
    re.compile(r"{%-?\s*raw\s*-?%}.*?{%-?\s*endraw\s*-?%}", re.S),  # raw blocks
    re.compile(r"<!--.*?-->", re.S),  # HTML comments
    re.compile(r"<a\b[^>]*>.*?</a>", re.S | re.I),  # existing HTML links
    re.compile(r"{%.*?%}", re.S),  # Liquid tags
    re.compile(r"{{.*?}}", re.S),  # Liquid output
    re.compile(r"<[a-zA-Z/!][^>]*>", re.S),  # HTML tags
    re.compile(r"(`+)[^`\n]*?\1"),  # inline code
    re.compile(r"!?\[[^\]\n]*\]\([^)\n]*\)"),  # Markdown links and images
    re.compile(r"\[[^\]\n]*\]\[[^\]\n]*\]"),  # reference links
    re.compile(r"^\[[^\]\n]+\]:.*$", re.M),  # reference definitions
    re.compile(r"^#{1,6} .*$", re.M),  # headings
    re.compile(r"^\{:.*\}[ \t]*$", re.M),  # kramdown attribute lines
    re.compile(r"https?://[^\s<>)\]]+"),  # bare URLs
]
H2_RE = re.compile(r"^## .*$", re.M)
LINKED_RE = re.compile(r"glossary\.html#([a-z0-9-]+)")
LANG_RE = re.compile(r"^lang:\s*(\w+)", re.M)
PERMALINK_RE = re.compile(r"^permalink:\s*[\"']?([^\"'\n]+?)[\"']?\s*$", re.M)


def load_patterns(path, glossary_dir):
    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    skip = set(data.get("skip") or [])
    terms = data["terms"]
    known = {p.name.removesuffix("_RU.md").removesuffix(".md") for p in glossary_dir.glob("*.md")}
    missing = known - set(terms) - skip
    unknown = (set(terms) | skip) - known
    if missing or unknown:
        sys.exit(f"term-patterns.yaml is out of sync with {glossary_dir}: "
                 f"missing {sorted(missing)}, unknown {sorted(unknown)}")
    compiled = {"en": [], "ru": []}
    for term_id, langs in terms.items():
        if term_id in skip:
            continue
        paths = tuple(langs.get("paths") or ())
        for lang in compiled:
            for pattern in langs.get(lang) or []:
                regex = re.compile(rf"(?<![\w-])(?:{pattern})(?![\w-])", re.I)
                compiled[lang].append((term_id, regex, paths))
    return compiled


BLOCK_TAG_RE = re.compile(r"<(/?)(div|table|details|summary|p|ul|ol|dl|blockquote|section|figure)\b[^>]*>", re.I)


def html_block_spans(text):
    """Spans of block-level HTML elements: kramdown renders no Markdown inside them."""
    spans = []
    depth = 0
    start = None
    for m in BLOCK_TAG_RE.finditer(text):
        closing = m.group(1) == "/"
        if not closing:
            if depth == 0:
                start = m.start()
            depth += 1
        elif depth > 0:
            depth -= 1
            if depth == 0:
                spans.append((start, m.end()))
    if depth > 0 and start is not None:
        spans.append((start, len(text)))
    return spans


def protected_spans(text):
    spans = html_block_spans(text)
    for regex in PROTECTED_RES:
        spans.extend(m.span() for m in regex.finditer(text))
    spans.sort()
    merged = []
    for start, end in spans:
        if merged and start <= merged[-1][1]:
            merged[-1] = (merged[-1][0], max(merged[-1][1], end))
        else:
            merged.append((start, end))
    return merged


def in_spans(pos, spans):
    return any(start <= pos < end for start, end in spans)


def gaps(spans, start, end):
    pos = start
    for s, e in spans:
        if e <= start:
            continue
        if s >= end:
            break
        if s > pos:
            yield pos, s
        pos = max(pos, e)
    if pos < end:
        yield pos, end


def glossary_href(permalink):
    path = re.sub(r"^(en|ru)/", "", permalink.strip("/") + ("/" if permalink.endswith("/") else ""))
    parts = [p for p in path.split("/") if p]
    depth = len(parts) if path.endswith("/") else len(parts) - 1
    return "../" * max(depth, 0) + "glossary.html"


def link_page(text, patterns, href, min_distance, max_line_length=MAX_LINE_LENGTH):
    """Return (new_text, edits) where edits is a list of (term_id, matched_text, position).

    A term is linked at its first mention in a section. In later sections it is
    linked again only if the previous link to it is at least ``min_distance``
    lines above, so short pages get one link per term and long pages get a
    link where the reader is likely to have scrolled past the previous one.
    Distances are measured in lines because inserting links does not change
    them, which keeps repeated runs from adding links near existing ones.
    """
    spans = protected_spans(text)
    newlines = [i for i, ch in enumerate(text) if ch == "\n"]

    def line_of(pos):
        return bisect.bisect_left(newlines, pos)

    heads = [m.start() for m in H2_RE.finditer(text)
             if not any(s <= m.start() < e and text[s:e].lstrip()[:3] in ("```", "~~~") for s, e in spans)]
    bounds = [0] + heads + [len(text)]
    replacements = []
    last_link = {}  # term_id -> line of the previous link (existing or new)
    existing = [(m.start(), m.group(1)) for m in LINKED_RE.finditer(text)]
    seen = 0  # existing links before the current position are folded into last_link
    for sec_start, sec_end in zip(bounds, bounds[1:]):
        section = text[sec_start:sec_end]
        linked = set(LINKED_RE.findall(section))
        for gap_start, gap_end in gaps(spans, sec_start, sec_end):
            pos = gap_start
            while pos < gap_end:
                while seen < len(existing) and existing[seen][0] < pos:
                    last_link[existing[seen][1]] = line_of(existing[seen][0])
                    seen += 1
                best = None
                for term_id, regex, _ in patterns:
                    if term_id in linked:
                        continue
                    if term_id in last_link and line_of(pos) - last_link[term_id] < min_distance:
                        continue
                    m = regex.search(text, pos, gap_end)
                    if m and (best is None or (m.start(), -(m.end() - m.start())) < (best[1].start(), -(best[1].end() - best[1].start()))):
                        best = (term_id, m)
                if best is None:
                    break
                term_id, m = best
                line_start = text.rfind("\n", 0, m.start()) + 1
                line_end = text.find("\n", m.end())
                line_end = len(text) if line_end == -1 else line_end
                added = sum(len(f"[]({href}#{t})") for st, _, t, _ in replacements if line_start <= st < line_end)
                if line_end - line_start + added + len(f"[]({href}#{term_id})") > max_line_length:
                    # markdownlint MD013 would reject the line; look for the next mention instead
                    pos = m.end()
                    continue
                linked.add(term_id)
                last_link[term_id] = line_of(m.start())
                replacements.append((m.start(), m.end(), term_id, m.group(0)))
                pos = m.end()
    out = text
    for start, end, term_id, matched in sorted(replacements, reverse=True):
        out = f"{out[:start]}[{matched}]({href}#{term_id}){out[end:]}"
    return out, [(t, s, st) for st, _, t, s in sorted(replacements)]


def page_files(pages_dir):
    for path in sorted(pages_dir.rglob("*.md")):
        rel = path.relative_to(pages_dir)
        if rel.parts[0] in EXCLUDED_DIRS or path.name in EXCLUDED_FILES:
            continue
        yield path


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--root", default=".", help="repository root")
    parser.add_argument("--pages", default=DEFAULT_PAGES, help="pages directory relative to root")
    parser.add_argument("--patterns", default=str(DEFAULT_PATTERNS))
    parser.add_argument("--apply", action="store_true", help="write changes (default: report only)")
    parser.add_argument("--show", type=int, default=0, help="print this many sample replacements per term")
    parser.add_argument("--min-distance", type=int, default=60,
                        help="lines between repeated links of one term on a page (default 60)")
    parser.add_argument("--context", default="", help="comma-separated term ids: print every match with context")
    args = parser.parse_args()
    context_terms = {t for t in args.context.split(",") if t}

    root = pathlib.Path(args.root).resolve()
    patterns = load_patterns(pathlib.Path(args.patterns), root / DEFAULT_GLOSSARY)
    per_term = collections.Counter()
    per_page = {}
    samples = collections.defaultdict(list)
    skipped = []
    for path in page_files(root / args.pages):
        text = path.read_text(encoding="utf-8")
        fm = FRONT_MATTER_RE.match(text)
        if not fm:
            skipped.append((path, "no front matter"))
            continue
        permalink = PERMALINK_RE.search(fm.group(1))
        if not permalink:
            skipped.append((path, "no permalink"))
            continue
        lang_m = LANG_RE.search(fm.group(1))
        lang = lang_m.group(1) if lang_m else ("ru" if path.name.endswith("_RU.md") else "en")
        if lang not in patterns:
            skipped.append((path, f"unsupported lang {lang}"))
            continue
        head, body = text[:fm.end()], text[fm.end():]
        rel_page = str(path.relative_to(root / args.pages))
        page_patterns = [p for p in patterns[lang] if not p[2] or rel_page.startswith(p[2])]
        new_body, edits = link_page(body, page_patterns, glossary_href(permalink.group(1)), args.min_distance)
        if not edits:
            continue
        per_page[str(path.relative_to(root))] = len(edits)
        for term_id, matched, position in edits:
            per_term[term_id] += 1
            if len(samples[term_id]) < args.show:
                samples[term_id].append((path.name, matched))
            if term_id in context_terms:
                ctx = body[max(0, position - 60):position + len(matched) + 40].replace("\n", " ")
                print(f"{term_id:22s} {rel_page}: ...{ctx}...")
        if args.apply:
            path.write_text(head + new_body, encoding="utf-8")

    total = sum(per_page.values())
    print(f"{'Applied' if args.apply else 'Would add'} {total} links in {len(per_page)} pages; {len(per_term)} terms used.")
    if per_page:
        counts = sorted(per_page.values())
        print(f"Links per page: median {counts[len(counts) // 2]}, max {counts[-1]}.")
        print("Pages with most links:")
        for page, n in sorted(per_page.items(), key=lambda kv: -kv[1])[:10]:
            print(f"  {n:4d}  {page}")
    print("Links per term:")
    for term_id, n in per_term.most_common():
        print(f"  {n:4d}  {term_id}")
    if args.show:
        print("Samples:")
        for term_id in sorted(samples):
            for name, matched in samples[term_id]:
                print(f"  {term_id:28s} {name:40s} {matched!r}")
    for path, reason in skipped:
        print(f"skipped {path}: {reason}", file=sys.stderr)


if __name__ == "__main__":
    main()
