#!/usr/bin/python3
# -*- coding: utf-8 -*-

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

import os
import re
import subprocess
import sys
import tempfile
import yaml
from bs4 import BeautifulSoup
from datetime import datetime
from typing import Any, List, Dict, Optional, Tuple

# Languages to build (order = PDF generation order).
SUPPORTED_LANGS: Tuple[str, ...] = ("ru", "en")

SIDEBAR_YAML = "main.yml"

# H1 before embedded-modules block in merged HTML.
MODULES_SECTION_H1: Dict[str, str] = {
    "ru": "Встроенные модули Deckhouse Kubernetes Platform",
    "en": "Embedded Deckhouse Kubernetes Platform modules",
}


def intermediate_html_path(lang: str) -> str:
    return f"extracted_content_{lang}.html"


def intermediate_html_dir(lang: str) -> str:
    return f"chunks_{lang}"


# Max uncompressed HTML bytes per chunk. Qt WebKit scales fonts down when a
# single HTML document exceeds ~2–3 MB; keeping each chunk well below that
# threshold ensures consistent font rendering across documents of very
# different sizes (e.g. admin guide vs. user guide).
_CHUNK_MAX_BYTES = 1_000_000  # 1 MB


def pdf_output_path_for_lang(base_pdf_path: str, lang: str) -> str:
    base, ext = os.path.splitext(base_pdf_path)
    if not ext:
        ext = ".pdf"
    return f"{base}_{lang}{ext}"


def content_base_path(lang: str) -> str:
    return os.path.join("content", lang)


def embedded_modules_root(lang: str) -> str:
    return os.path.join("embedded-modules", lang, "modules")


def traverse_menu_to_list(
    yaml_file_path: str,
    lang: str,
    only_top_section: Optional[str] = None,
    exclude_top_sections: Optional[List[str]] = None,
) -> List[Dict[str, Optional[str]]]:
    """Collects sidebar entries with localized title (lang, fallback ru/en, or plain string).

    When only_top_section is set, only entries under the matching top-level section are returned.
    When exclude_top_sections is set, entries under matching top-level sections are skipped.
    The match is checked against both EN and RU title values of level-0 entries.
    """
    with open(yaml_file_path, "r", encoding="utf-8") as f:
        data = yaml.safe_load(f)

    results: List[Dict[str, Optional[str]]] = []

    def get_localized_title(obj: Any) -> str:
        if isinstance(obj, dict):
            title = obj.get("title")
            if isinstance(title, dict):
                return (
                    (title.get(lang) or "").strip()
                    or (title.get("ru") or "").strip()
                    or (title.get("en") or "").strip()
                )
            if isinstance(title, str):
                return title
        return ""

    _exclude = set(exclude_top_sections) if exclude_top_sections else set()

    def walk(node: Any, level: int, in_section: Optional[bool] = None) -> None:
        if isinstance(node, list):
            for item in node:
                walk(item, level, in_section)
        elif isinstance(node, dict):
            if node.get("draft"):
                return
            if level == 0:
                raw = node.get("title", {})
                en_title = (raw.get("en") or "").strip() if isinstance(raw, dict) else ""
                ru_title = (raw.get("ru") or "").strip() if isinstance(raw, dict) else ""
                if only_top_section is not None:
                    in_section = (en_title == only_top_section or ru_title == only_top_section)
                elif _exclude and (en_title in _exclude or ru_title in _exclude):
                    in_section = False
            if in_section is None or in_section:
                title_loc = get_localized_title(node)
                if title_loc:
                    url = node.get("url")
                    results.append({
                        "title": title_loc,
                        "level": level,
                        "url": url
                    })
                folders = node.get("folders")
                if folders is not None:
                    walk(folders, level + 1, in_section)
            else:
                folders = node.get("folders")
                if folders is not None:
                    walk(folders, level + 1, False)

    entries = data.get("entries", [])
    walk(entries, level=0)
    return results


def generate_html_header(title: str = "Extracted content", lang: str = "ru") -> str:
    """Generates HTML header."""
    html_lang = "ru" if lang == "ru" else "en"
    html = f"""<!DOCTYPE html>
<html lang="{html_lang}">
<head>
    <meta charset="utf-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>""" + title + """</title>
    <style>
        body {
            font-size: 11pt;
        }
        p, li, td, th, dt, dd, blockquote, figcaption, label, span, div {
            font-size: 11pt;
        }
        h1 { font-size: 18pt; }
        h2 { font-size: 16pt; }
        h3 { font-size: 14pt; }
        h4 { font-size: 12pt; }
        h5, h6 { font-size: 11pt; }
        pre, code, kbd, samp, tt {
            font-family: "DejaVu Sans Mono", "Courier New", Courier, monospace;
        }
        pre {
            font-size: 9pt;
        }
        code {
            font-size: inherit;
        }
        a, a:link, a:visited, a:hover, a:active {
            color: black;
            text-decoration: none;
        }
        /* Таблицы: не шире области печати, перенос длинного текста в ячейках.
           border-separate + border-spacing:0 вместо collapse — иначе wkhtmltopdf
           дублирует верхнюю границу строки при переносе на следующую страницу. */
        table {
            border-collapse: separate;
            border-spacing: 0;
            border: none;
            max-width: 100% !important;
            width: 100% !important;
            table-layout: fixed;
            box-sizing: border-box;
        }
        /* Таблицы supported_versions — auto по умолчанию; для Linux-таблицы
           постобработка ставит inline style table-layout:fixed + colgroup,
           который перебьёт это правило (без !important). */
        table.supported_versions {
            table-layout: auto;
        }
        table th,
        table td {
            border-top: 1px solid black;
            border-left: 1px solid black;
            border-bottom: none;
            border-right: none;
            padding: 6px 8px;
            vertical-align: top;
            box-sizing: border-box;
        }
        /* Правые границы — только у последней ячейки в строке */
        table th:last-child,
        table td:last-child {
            border-right: 1px solid black;
        }
        /* Нижние границы — только у последней строки */
        table tr:last-child th,
        table tr:last-child td {
            border-bottom: 1px solid black;
        }
        /* Ячейки данных — перенос по словам, при необходимости по символам */
        table td {
            word-wrap: break-word;
            overflow-wrap: break-word;
            word-break: break-word;
        }
        /* Заголовки — перенос только по словам, не по буквам */
        table th {
            background-color: #f0f0f0;
            word-break: normal;
            overflow-wrap: normal;
            hyphens: none;
            white-space: normal;
        }
        /* Таблицы supported_versions — центрировать ячейки данных */
        table.supported_versions td {
            text-align: center;
        }
        table.supported_versions td.name,
        table.supported_versions td.versions {
            text-align: left;
        }
        /* Вписываем иллюстрации в ширину страницы PDF (wkhtmltopdf обрезал широкие img) */
        img,
        svg,
        video,
        object,
        embed {
            max-width: 100% !important;
            height: auto !important;
            box-sizing: border-box;
        }
        /* inline-SVG и картинки внутри типичных обёрток документации */
        picture img {
            max-width: 100% !important;
            height: auto !important;
        }
        figure {
            max-width: 100%;
            margin-left: 0;
            margin-right: 0;
        }
        /* Блоки кода (Rouge/Jekyll): визуальное отделение от текста + перенос строк */
        pre,
        pre.highlight,
        .highlight pre,
        div.highlighter-rouge pre,
        div.highlight pre {
            background-color: #f6f8fa;
            border: 1px solid #d0d7de;
            border-radius: 4px;
            padding: 10px 12px;
            margin: 8px 0;
            white-space: pre-wrap !important;
            word-wrap: break-word;
            overflow-wrap: break-word;
            word-break: break-word;
            max-width: 100% !important;
            box-sizing: border-box;
            page-break-inside: avoid;
            line-height: 1.4;
        }
        pre code,
        .highlight code {
            background-color: transparent;
            border: none;
            padding: 0;
            white-space: pre-wrap !important;
            word-break: break-word;
        }
        div.highlighter-rouge,
        div.highlight,
        div[class*="language-"] {
            max-width: 100%;
            box-sizing: border-box;
        }
        /* Инлайн-код — фон и padding; pre code сбрасывается отдельно выше */
        code {
            background-color: #efefef;
            padding: 1px 3px;
            overflow-wrap: break-word;
            word-break: break-word;
        }
        /* Alert-блоки */
        div.alert__wrap {
            border-left: 4px solid #004df2;
            background-color: #f0f4ff;
            padding: 8px 12px 8px 32px;
            margin: 10px 0;
            border-radius: 0 4px 4px 0;
            page-break-inside: avoid;
            position: relative;
        }
        div.alert__wrap.warning {
            border-left-color: #e6a700;
            background-color: #fef9e7;
        }
        div.alert__wrap.danger {
            border-left-color: #ff4141;
            background-color: #fff0f0;
        }
        span.alert-icon {
            position: absolute;
            left: 8px;
            top: 8px;
            font-size: 14pt;
            line-height: 1.2;
        }
        div.alert__wrap.danger span.alert-icon {
            color: #ff4141;
        }
        div.alert__wrap.warning span.alert-icon {
            color: #e6a700;
        }
        /* Blockquote (цитата) — аналогично info alert */
        blockquote.pdf-blockquote {
            border-left: 4px solid #004df2;
            background-color: #f0f4ff;
            padding: 8px 12px 8px 32px;
            margin: 10px 0;
            border-radius: 0 4px 4px 0;
            page-break-inside: avoid;
            position: relative;
        }
    </style>
</head>
<body>
"""

    return html


def extract_content_from_html(html_path: str) -> Optional[str]:
    """Extracts text from <div class="content"> in HTML file."""
    if not os.path.isfile(html_path):
        return None
    try:
        with open(html_path, "r", encoding="utf-8") as f:
            soup = BeautifulSoup(f, "html.parser")
        content_div = soup.find("div", class_="docs")
        if content_div:
            return str(content_div)
        return None
    except Exception as e:
        print(f"Error reading {html_path}: {e}")
        return None


def _strip_cache_buster_from_local_href_src(content: str) -> str:
    """Remove ?query from local href/src — file:// URLs with ?v=... often fail in wkhtmltopdf/Qt."""
    pattern = re.compile(
        r'(?P<prefix>\b(?:href|src)\s*=\s*)(?P<q>["\'])(?P<path>[^"\']+)(?P<query>\?[^"\']+)(?P=q)',
        re.IGNORECASE,
    )

    def repl(m: re.Match[str]) -> str:
        path = m.group("path")
        if path.startswith(("http://", "https://", "mailto:", "data:", "//")):
            return m.group(0)
        return f'{m.group("prefix")}{m.group("q")}{path}{m.group("q")}'

    return pattern.sub(repl, content)


def fix_image_paths(
    content: str, apply_special_path_rewrites: bool = True, lang: str = "ru"
) -> str:
    """Fixes image, stylesheet and other resource paths in HTML for merged extracted_content.html.

    For fragments already prefixed with embedded-modules/.../MODULE/images/, set
    apply_special_path_rewrites=False: special_paths uses a ``/images/`` alternative that
    would otherwise consume the slash after the module slug (…/log-shipper + /images/ →
    …/log-shippercontent/images/).
    """
    if not content:
        return content

    # --- /modules/ and file:///modules/ resolve from FS root in wkhtmltopdf → map to embedded-modules tree
    mod_fs_prefix = f"embedded-modules/{lang}/modules/"
    content = re.sub(
        r"file:///modules/", mod_fs_prefix, content, flags=re.IGNORECASE
    )
    content = re.sub(
        r'((?:src|href|xlink:href)\s*=\s*["\'])/modules/',
        rf"\1{mod_fs_prefix}",
        content,
        flags=re.IGNORECASE,
    )

    # --- Relative ../.../images/ from source pages become wrong when merged into
    # extracted_content.html at repo root: ../../images/ resolves to /home/<user>/images/
    # instead of content/images/. Normalize to project-relative content/images/.
    content = re.sub(
        r'((?:src|href|xlink:href)\s*=\s*)(["\'])(?:\.\./)+images/',
        r"\1\2content/images/",
        content,
        flags=re.IGNORECASE,
    )

    # --- Wrong absolute paths (same symptom in wkhtmltopdf logs)
    content = re.sub(r"file:///home/[^/\s\"']+/images/", "content/images/", content)
    content = re.sub(r"file:///home/images/", "content/images/", content)
    content = re.sub(
        r'((?:src|href|xlink:href)\s*=\s*["\'])/home/[^/\s"\']+/images/',
        r"\1content/images/",
        content,
        flags=re.IGNORECASE,
    )
    content = re.sub(
        r'((?:src|href|xlink:href)\s*=\s*["\'])/home/images/',
        r"\1content/images/",
        content,
        flags=re.IGNORECASE,
    )

    # --- ../assets/ and /assets/ → content/assets/ (same ../ resolution bug as for images)
    content = re.sub(
        r'((?:src|href)\s*=\s*)(["\'])(?:\.\./)+assets/',
        r"\1\2content/assets/",
        content,
        flags=re.IGNORECASE,
    )
    content = re.sub(
        r'((?:src|href)\s*=\s*)(["\'])/assets/',
        r"\1\2content/assets/",
        content,
        flags=re.IGNORECASE,
    )
    content = re.sub(
        r"file:///home(?:/[^/\s\"']+)*/assets/",
        "content/assets/",
        content,
    )

    # Dictionary for special paths: filename -> subdirectory
    special_paths = {
        # virtualization-platform
        'vd-immediate': 'virtualization-platform',
        'vd-wffc': 'virtualization-platform',
        'vm-lifecycle': 'virtualization-platform',
        'vm-corefraction': 'virtualization-platform',
        'placement-nodeselector': 'virtualization-platform',
        'placement-node-affinity': 'virtualization-platform',
        'placement-vm-affinity': 'virtualization-platform',
        'placement-vm-antiaffinity': 'virtualization-platform',
        'lb-nodeport': 'virtualization-platform',
        'lb-loadbalancer': 'virtualization-platform',
        'lb-ingress': 'virtualization-platform',
        'migration': 'virtualization-platform',
        'vm-restore-clone': 'virtualization-platform',
        'cases-vms': 'virtualization-platform',
        'cases-pods-and-vms': 'virtualization-platform',
        'cases.dkp': 'virtualization-platform',
        'arch': 'virtualization-platform',
        'vm': 'virtualization-platform',
        'vmclass-examples': 'virtualization-platform',
        'drain': 'virtualization-platform',
        'coldstandby': 'virtualization-platform',
        # istio
        'istio-architecture': 'istio',
        # cni-cilium
        'dsr': 'cni-cilium',
        'snat': 'cni-cilium',
        # storage/sds/node-configurator
        'sds-node-configurator-scenaries': 'storage/sds/node-configurator',
        # log-shipper
        'log_shipper_architecture': 'log-shipper',
        'log_shipper_distributed': 'log-shipper',
        'log_shipper_centralized': 'log-shipper',
        'log_shipper_stream': 'log-shipper',
        'log_shipper_pipeline': 'log-shipper',
        'grafana_cloud': 'log-shipper',
        # prometheus
        'prometheus_monitoring': 'prometheus',
        # operator-prometheus
        'targets': 'operator-prometheus',
        'pod': 'operator-prometheus',
        'servicemonitors': 'operator-prometheus',
        'rules': 'operator-prometheus',
        # stronghold
        'admin-guide-image1': 'stronghold',
        'admin-guide-image2': 'stronghold',
        'admin-guide-image3': 'stronghold',
        'admin-guide-image4': 'stronghold',
        'admin-guide-image5': 'stronghold',
        'image2.ru': 'stronghold',
        # upmeter
        'image1': 'upmeter',
        'image2': 'upmeter',
        # runtime-audit-engine
        'falco_daemonset': 'runtime-audit-engine',
        'falco_pod': 'runtime-audit-engine',
        'falco_shop': 'runtime-audit-engine',
        # user-authn
        'dex_login': 'user-authn',
        'kubeconfig_dex': 'user-authn',
    }

    # First, fix special paths (skip for embedded-module HTML: breaks …/MODULE/images/foo paths)
    if apply_special_path_rewrites:
        for img_name, subdir in special_paths.items():
            escaped_img_name = re.escape(img_name)
            escaped_subdir = re.escape(subdir)
            pattern = (
                r'(?<!content/images/'
                + escaped_subdir
                + r'/)(content/images/|images/|/images/)'
                + escaped_img_name
                + r'(\.ru)?(\.png|\.svg|\.jpg|\.jpeg|\.gif)'
            )
            replacement = f'content/images/{subdir}/{img_name}\\2\\3'
            content = re.sub(pattern, replacement, content)

    # Then fix general paths: /images/ -> content/images/
    content = re.sub(r'src="/images/', 'src="content/images/', content)
    content = re.sub(r'xlink:href="/images/', 'xlink:href="content/images/', content)
    content = re.sub(r'href="/images/', 'href="content/images/', content)

    # Fix relative paths: images/ or ./images/ -> content/images/ (main site content only)
    if apply_special_path_rewrites:
        content = re.sub(r'src="(?<!content/)images/', 'src="content/images/', content)
        content = re.sub(r'src="\./images/', 'src="content/images/', content)
        content = re.sub(r'xlink:href="(?<!content/)images/', 'xlink:href="content/images/', content)
        content = re.sub(r'xlink:href="\./images/', 'xlink:href="content/images/', content)
        content = re.sub(r'href="(?<!content/)images/', 'href="content/images/', content)
        content = re.sub(r'href="\./images/', 'href="content/images/', content)

        content = re.sub(r"src='/images/", "src='content/images/", content)
        content = re.sub(r"xlink:href='/images/", "xlink:href='content/images/", content)
        content = re.sub(r"href='/images/", "href='content/images/", content)
        content = re.sub(r"src='(?<!content/)images/", "src='content/images/", content)
        content = re.sub(r"src='\./images/", "src='content/images/", content)
        content = re.sub(r"xlink:href='(?<!content/)images/", "xlink:href='content/images/", content)
        content = re.sub(r"xlink:href='\./images/", "xlink:href='content/images/", content)
        content = re.sub(r"href='(?<!content/)images/", "href='content/images/", content)
        content = re.sub(r"href='\./images/", "href='content/images/", content)
    else:
        # Still rewrite absolute /images/ for single-quoted attributes
        content = re.sub(r"src='/images/", "src='content/images/", content)
        content = re.sub(r"xlink:href='/images/", "xlink:href='content/images/", content)
        content = re.sub(r"href='/images/", "href='content/images/", content)

    # Fix erroneous paths .content/images/ -> content/images/
    content = re.sub(r'\.content/images/', 'content/images/', content)

    # Fix paths for stronghold files: remove .ru from name if file doesn't exist
    # Files are named admin-guide-image*.png, not admin-guide-image*.ru.png
    content = re.sub(r'content/images/stronghold/(admin-guide-image\d+)\.ru\.png', r'content/images/stronghold/\1.png', content)
    # Also fix image2.ru.png -> image2.png (if image2.ru.png file doesn't exist)
    content = re.sub(r'content/images/stronghold/image2\.ru\.png', r'content/images/stronghold/image2.png', content)

    content = _strip_cache_buster_from_local_href_src(content)

    return content


def fix_embedded_module_relative_resource_paths(
    content: str, module_slug: str, lang: str = "ru"
) -> str:
    """Module pages use images/foo.svg relative to the module dir; merge file is at repo root."""
    if not content:
        return content
    prefix = f"embedded-modules/{lang}/modules/{module_slug}/"
    # src="images/..." or src="./images/..." (not http(s), /, embedded-modules/, content/)
    pat = re.compile(
        r'(\b(?:src|href|xlink:href)\s*=\s*)(["\'])((?:\./)?images/)',
        re.IGNORECASE,
    )

    def repl(m: re.Match[str]) -> str:
        return f"{m.group(1)}{m.group(2)}{prefix}{m.group(3)}"

    return pat.sub(repl, content)


def embedded_module_html_sort_key(filename: str) -> Tuple[int, str]:
    """Order: Описание → Настройки → CRD → Использование → FAQ → остальные (по имени)."""
    n = filename.lower()
    if n == "index.html":
        return (0, "")
    if n == "configuration.html":
        return (1, "0")
    if n == "cluster_configuration.html":
        return (1, "1")
    if n == "environment.html":
        return (1, "2")
    if n == "cr.html":
        return (2, "0")
    if n.endswith("-cr.html"):
        return (2, "1" + n)
    if n == "usage.html":
        return (3, "")
    if n == "faq.html":
        return (4, "")
    return (5, n)


def demote_headings_one_level(soup: BeautifulSoup) -> None:
    """h1→h2, …, h5→h6; h6 без изменений (чтобы не терять уровни в глубокой вложенности)."""
    for old_level in range(6, 0, -1):
        new_level = min(old_level + 1, 6)
        new_name = f"h{new_level}"
        for tag in list(soup.find_all(f"h{old_level}")):
            tag.name = new_name


def _extract_module_base_title_from_index_path(index_path: str) -> Optional[str]:
    """Название модуля из index.html (docs__title), для сокращения заголовков подстраниц."""
    if not os.path.isfile(index_path):
        return None
    try:
        with open(index_path, "r", encoding="utf-8") as f:
            soup = BeautifulSoup(f, "html.parser")
        root = soup.find("div", class_="docs") or soup
        h1 = root.find("h1", class_=lambda c: c and "docs__title" in c)
        if not h1:
            return None
        return " ".join(h1.get_text().split())
    except OSError as e:
        print(f"Warning: could not read module index for title: {index_path}: {e}")
        return None


def _strip_repeated_module_title_from_subpage(full_title: str, module_base: str) -> Optional[str]:
    """
    Если заголовок подстраницы начинается с того же названия, что и модуль, и затем разделитель
    (:, —, -), возвращает остаток с заглавной первой буквой. Иначе None.
    """
    full_n = " ".join(full_title.split())
    base_n = " ".join(module_base.split())
    if not base_n or len(full_n) <= len(base_n):
        return None
    if full_n[: len(base_n)].casefold() != base_n.casefold():
        return None
    rest = full_n[len(base_n) :]
    rest = re.sub(r"^\s*[:,—–\-]\s*", "", rest, count=1).strip()
    if not rest:
        return None
    return rest[0].upper() + rest[1:]


_HEADING_TAG_RE = re.compile(r"^h[1-6]$", re.IGNORECASE)


def _heading_level_from_tag(tag: Any) -> int:
    name = getattr(tag, "name", None) or ""
    if len(name) == 2 and name.lower().startswith("h") and name[1].isdigit():
        return int(name[1])
    return 99


def remove_sections_with_exact_heading(soup: BeautifulSoup, exact_title: str) -> None:
    """Удаляет заголовок и узлы-сиблинги до следующего заголовка того же или более высокого уровня."""
    want = " ".join(exact_title.split())
    while True:
        found: Optional[Any] = None
        for hn in soup.find_all(_HEADING_TAG_RE):
            if " ".join(hn.get_text().split()) == want:
                found = hn
                break
        if found is None:
            break
        lvl = _heading_level_from_tag(found)
        to_remove: List[Any] = [found]
        cur = found.next_sibling
        while cur is not None:
            if getattr(cur, "name", None) and _HEADING_TAG_RE.match(str(cur.name)):
                if _heading_level_from_tag(cur) <= lvl:
                    break
            to_remove.append(cur)
            cur = cur.next_sibling
        for node in to_remove:
            if hasattr(node, "decompose"):
                node.decompose()
            else:
                node.extract()


def postprocess_extracted_docs_soup(soup: BeautifulSoup, lang: str) -> None:
    """In-place cleanup of extracted <div class=\"docs\"> fragment (main.yml + embedded, per locale)."""
    page_nav_divs = soup.find_all("div", class_="page-navigation")
    for div in page_nav_divs:
        div.decompose()

    for btn in soup.find_all("button", class_=lambda c: c and "show__containers" in c):
        btn.decompose()

    # Баннер про «документацию ещё не вышедшей версии Deckhouse» / EN prerelease notice
    for div in soup.find_all("div", id="notice-latest-doc-version-block"):
        div.decompose()
    if lang == "en":
        prerelease_markers = (
            "not yet been released",
            "has not been released",
            "preliminary documentation",
        )
    else:
        prerelease_markers = ("не вышедшей версии Deckhouse",)
    for div in soup.find_all(
        "div",
        class_=lambda c: c and "alert__wrap" in c and "warning" in c,
    ):
        p_text = " ".join(div.get_text().split())
        if any(m in p_text for m in prerelease_markers):
            div.decompose()

    # alert-блоки про доступность в редакциях / Available in editions
    for div in list(
        soup.find_all(
            "div",
            class_=lambda c: c
            and "alert__wrap" in c
            and ("info" in c or "warning" in c),
        )
    ):
        t = " ".join(div.get_text().split())
        if lang == "en":
            tl = t.lower()
            if "available" in tl and "edition" in tl:
                div.decompose()
        elif "Доступно" in t and (
            "в редакциях" in t or "следующих редакциях" in t
        ):
            div.decompose()

    # Блок «Стадия жизненного цикла модуля» / Module lifecycle (info alert)
    if lang == "en":
        lc_href_markers = ("module-lifecycle", "жизненный-цикл-модуля")
        lc_link_markers = ("Module lifecycle", "Lifecycle stage")
    else:
        lc_href_markers = ("жизненный-цикл-модуля", "module-lifecycle")
        lc_link_markers = ("Стадия жизненного цикла",)
    for a in list(soup.find_all("a", href=True)):
        attrs_a = getattr(a, "attrs", None)
        if not isinstance(attrs_a, dict):
            continue
        href = str(attrs_a.get("href") or "")
        if not any(m in href for m in lc_href_markers):
            continue
        link_text = " ".join(a.get_text().split())
        if not any(m in link_text for m in lc_link_markers):
            continue
        block = a.find_parent(
            "div",
            class_=lambda c: c and "alert__wrap" in c and "info" in c,
        )
        if block is not None:
            block.decompose()

    info_alert_divs = soup.find_all("div", class_="info alert__wrap")
    for div in info_alert_divs:
        paragraphs = div.find_all("p")
        for p in paragraphs:
            p_text = " ".join(p.get_text().split())
            if lang == "en":
                targets = (
                    "Links to related documentation and API:",
                    "Links to related documentation",
                )
            else:
                targets = ("Ссылки на связанную документацию и API:",)
            if any(target_text in p_text for target_text in targets):
                div.decompose()
                break

    _alert_symbols = {"danger": "✖", "warning": "⚠", "info": "ℹ"}
    for div in soup.find_all("div", class_="alert__wrap"):
        for svg in div.find_all("svg", class_="alert__icon"):
            svg.decompose()
        classes = div.get("class", [])
        for level, symbol in _alert_symbols.items():
            if level in classes:
                icon_span = soup.new_tag("span", attrs={"class": "alert-icon"})
                icon_span.string = symbol
                div.insert(0, icon_span)
                break

    for bq in soup.find_all("blockquote"):
        bq["class"] = bq.get("class", []) + ["pdf-blockquote"]
        icon_span = soup.new_tag("span", attrs={"class": "alert-icon"})
        icon_span.string = "ℹ"
        bq.insert(0, icon_span)

    plus_icon_spans = soup.find_all("span", class_="plus-icon")
    for span in plus_icon_spans:
        span.decompose()

    minus_icon_spans = soup.find_all("span", class_="minus-icon")
    for span in minus_icon_spans:
        span.decompose()

    details_divs = soup.find_all("div", class_="details")
    for div in details_divs:
        links = div.find_all("a")
        for link in links:
            link_text = " ".join(link.get_text().split())
            if lang == "en":
                module_enable_markers = (
                    "How to explicitly enable or disable the module",
                    "How to explicitly enable",
                )
            else:
                module_enable_markers = ("Как явно включить или отключить модуль",)
            if any(m in link_text for m in module_enable_markers):
                link.decompose()
                break

    ancestor_spans = soup.find_all("span", class_="ancestors")
    for span in ancestor_spans:
        parent_div = span.find_parent("div")
        if parent_div:
            parent_div.wrap(soup.new_tag("b"))

    font_tags = soup.find_all("font", {"size": "-1"})
    for font_tag in font_tags:
        font_tag.attrs.pop("size", None)
        if not font_tag.attrs:
            font_tag.unwrap()

    def _is_nested_in_tabs_container(tag: Any, container: Any) -> bool:
        """True if tag has a tabs__content ancestor between itself and container."""
        cur = tag.parent
        while cur is not None and cur is not container:
            cls = getattr(cur, "attrs", {}).get("class") or []
            if "tabs__content" in cls:
                return True
            cur = cur.parent
        return False

    tabs_divs = soup.find_all("div", class_="tabs")
    for tabs_div in tabs_divs:
        parent = tabs_div.parent
        if not parent:
            continue

        # Build label map: panel-id → button text (skip panels nested in sibling panels)
        btn_label: Dict[str, str] = {}
        for btn in tabs_div.find_all("a", class_=lambda x: x and "tabs__btn" in x):
            store_val = (btn.get("data-store-val") or "").strip()
            label = " ".join(btn.get_text().split())
            if store_val:
                for panel in parent.find_all(
                    "div",
                    class_=lambda x: x and "tabs__content" in x,
                    id=lambda pid: pid and pid.endswith("_" + store_val),
                ):
                    if not _is_nested_in_tabs_container(panel, parent):
                        btn_label[panel.get("id")] = label

        # Show all direct (non-nested) panels with a bold label; remove tab button bar
        panels = [
            p for p in parent.find_all("div", class_=lambda x: x and "tabs__content" in x)
            if not _is_nested_in_tabs_container(p, parent)
        ]
        for panel in panels:
            panel_id = panel.get("id") or ""
            label = btn_label.get(panel_id, "")
            # Make panel visible (remove display:none imposed by tabs__content CSS)
            panel["style"] = "display:block;"
            if label:
                # Find the first real child element to decide insertion strategy
                first_child = next(
                    (c for c in panel.children if getattr(c, "name", None)),
                    None,
                )
                if first_child and first_child.name in ("ol", "ul"):
                    # Panel starts with a list — fix the list's padding-left explicitly,
                    # then give the label the same indent so they align pixel-perfect.
                    _list_indent = "40px"
                    existing_style = (first_child.get("style") or "").rstrip(";")
                    first_child["style"] = (
                        (existing_style + "; " if existing_style else "")
                        + f"padding-left:{_list_indent};"
                    )
                    label_tag = soup.new_tag(
                        "div",
                        style=f"font-weight:bold; padding-left:{_list_indent}; margin:0.3em 0 0.1em 0;",
                    )
                    label_tag.string = label
                    panel.insert(0, label_tag)
                else:
                    label_tag = soup.new_tag(
                        "div",
                        style="font-weight:bold; margin:0.3em 0 0.1em 0;",
                    )
                    label_tag.string = label
                    panel.insert(0, label_tag)

        tabs_div.decompose()

    # Исправить layout таблиц supported_versions:
    # - убрать table-layout: fixed и colgroup с пиксельными ширинами
    # - убрать width из ячеек
    # - Linux-таблица: явные ширины колонок через colgroup
    # - revision-comparison: первая колонка шире, остальные поровну
    # - Kubernetes: удалить первую колонку с иконками (SVG не рендерятся в PDF)
    for table in soup.find_all("table", class_="supported_versions"):
        classes = table.get("class", [])
        is_revision = "table__small" in classes
        is_kubernetes = ("supported_versions__kubernetes" in classes
                         and "supported_versions__kubernetes-container" not in classes)
        is_linux = (not is_revision and not is_kubernetes
                    and "supported_versions__kubernetes-container" not in classes)

        # Убрать table-layout из инлайн-стиля (зададим ниже явно)
        inline = table.get("style", "")
        inline = re.sub(r"table-layout\s*:\s*\w+\s*;?\s*", "", inline).strip()
        if inline:
            table["style"] = inline
        elif "style" in table.attrs:
            del table["style"]

        # Убрать colgroup с пиксельными ширинами
        for colgroup in table.find_all("colgroup"):
            colgroup.decompose()

        # Убрать width и white-space из инлайн-стилей ячеек
        for cell in table.find_all(["th", "td"]):
            cell_style = cell.get("style", "")
            changed = False
            if "width:" in cell_style:
                cell_style = re.sub(r"width\s*:\s*[^;]+;?\s*", "", cell_style)
                changed = True
            if "white-space:" in cell_style:
                cell_style = re.sub(r"white-space\s*:\s*[^;]+;?\s*", "", cell_style)
                changed = True
            if changed:
                cell_style = cell_style.strip().rstrip(";").strip()
                if cell_style:
                    cell["style"] = cell_style
                elif "style" in cell.attrs:
                    del cell["style"]

        # Kubernetes-таблица: удалить первую колонку (иконки SVG — пустые в PDF)
        if is_kubernetes:
            for row in table.find_all("tr"):
                cells = row.find_all(["th", "td"])
                if cells:
                    cells[0].decompose()

        def _col_count(tbl: Any) -> int:
            """Считает реальное число колонок с учётом colspan."""
            max_cols = 0
            for row in tbl.find_all("tr"):
                cols = sum(int(c.get("colspan", 1)) for c in row.find_all(["th", "td"]))
                if cols > max_cols:
                    max_cols = cols
            return max_cols

        # Linux-таблица: дистрибутив(25%), версии(20%), CE(10%), CSE(15%), BE/SE/SE+/EE(15%), примечания(15%)
        if is_linux:
            col_count = _col_count(table)
            if col_count == 6:
                widths = ["25%", "20%", "10%", "15%", "15%", "15%"]
            elif col_count == 4:
                # EN-версия: дистрибутив(30%), версии(25%), редакции(25%), примечания(20%)
                widths = ["30%", "25%", "25%", "20%"]
            else:
                w = 100 // col_count
                widths = [f"{w}%"] * col_count
            colgroup = soup.new_tag("colgroup")
            for w in widths:
                col = soup.new_tag("col")
                col["style"] = f"width: {w};"
                colgroup.append(col)
            table.insert(0, colgroup)
            table["style"] = "table-layout: fixed;"

        # revision-comparison: первая колонка — модуль, остальные — редакции.
        # Короткие (CE, BE, SE, SE+, EE) — минимальная фиксированная ширина,
        # длинные (CSE Lite, CSE Pro) — больше.
        if is_revision:
            col_count = _col_count(table)
            if col_count > 1:
                header_row = table.find("tr")
                headers = header_row.find_all(["th", "td"]) if header_row else []
                other_count = col_count - 1
                short_w = 7
                long_threshold = 5
                other_texts = [h.get_text(strip=True) for h in headers[1:]]
                long_count = sum(1 for t in other_texts if len(t) > long_threshold)
                short_count = other_count - long_count
                short_total = short_count * short_w
                first_pct = 30
                long_w = (100 - first_pct - short_total) // max(long_count, 1) if long_count else 0
                widths = [first_pct]
                for t in other_texts:
                    widths.append(long_w if len(t) > long_threshold else short_w)
                adj = 100 - sum(widths)
                widths[0] += adj
                colgroup = soup.new_tag("colgroup")
                for w in widths:
                    col = soup.new_tag("col")
                    col["style"] = f"width: {w}%;"
                    colgroup.append(col)
                table.insert(0, colgroup)
                table["style"] = "table-layout: fixed;"

    # Предотвратить разрыв строк при переносе таблицы на следующую страницу.
    # border-separate используется вместо collapse, чтобы устранить дублирование границ.
    # page-break-inside: avoid на tr дополнительно запрещает разрыв строки пополам.
    for table in soup.find_all("table"):
        tbody = table.find("tbody")
        if not tbody:
            continue
        for tr in tbody.find_all("tr", recursive=False):
            existing = tr.get("style", "")
            tr["style"] = (existing + "; " if existing else "") + "page-break-inside: avoid;"

    # Инлайн-код: убираем все inline-стили и class, которые могут конфликтовать.
    # wkhtmltopdf добавляет лишний пробел для inline-элементов с background/border/font-family.
    for code in soup.find_all("code"):
        if code.find_parent("pre"):
            continue
        if "style" in code.attrs:
            del code["style"]

    # Заменить SVG-иконки статуса (supported/not_supported/intermediate) на текстовые символы.
    # wkhtmltopdf не загружает <image href=""> внутри <svg>, поэтому иконки пустые.
    # Порядок важен: not_supported проверяем раньше supported (иначе подстрока "supported" даст ложный матч).
    _svg_icon_map = [
        ("not_supported", "✗"),
        ("intermediate", "~"),
        ("supported", "✓"),
    ]
    for svg in list(soup.find_all("svg")):
        img_tag = svg.find("image")
        if img_tag is None:
            continue
        href = img_tag.get("href", "") or img_tag.get("xlink:href", "")
        symbol = None
        for key, sym in _svg_icon_map:
            if key in href:
                symbol = sym
                break
        if symbol is None:
            continue
        parent = svg.parent
        tippy = ""
        if parent and hasattr(parent, "get"):
            tippy = parent.get("data-tippy-content", "") or ""
        new_content = symbol
        if tippy:
            new_content += f'<br><small style="font-size:8pt; color:#444;">{tippy}</small>'
        svg.replace_with(BeautifulSoup(new_content, "html.parser"))

    # Раскрыть все нативные <details> — wkhtmltopdf не открывает их по умолчанию.
    for details in soup.find_all("details"):
        details["open"] = ""
        details["style"] = "display:block;"
        summary = details.find("summary")
        if summary:
            summary["style"] = "font-weight:bold; display:block; margin-bottom:0.3em;"

    if lang == "en":
        remove_sections_with_exact_heading(soup, "External components")
    else:
        remove_sections_with_exact_heading(soup, "Внешние компоненты")

    # OSS info page: remove logo images, bold titles
    for logo_div in soup.find_all("div", class_="oss__item-logo"):
        logo_div.decompose()
    for title_a in soup.find_all("a", class_="oss__item-title"):
        title_a["style"] = "font-weight:bold;"


class _ChunkWriter:
    """Accumulates content and flushes to numbered HTML chunk files under a directory."""

    def __init__(self, chunk_dir: str, lang: str, max_bytes: int = _CHUNK_MAX_BYTES) -> None:
        self._dir = chunk_dir
        self._lang = lang
        self._max = max_bytes
        self._buf: List[str] = []
        self._buf_bytes = 0
        self._index = 0
        self._paths: List[str] = []
        os.makedirs(chunk_dir, exist_ok=True)

    def write(self, fragment: str) -> None:
        encoded = len(fragment.encode("utf-8"))
        if self._buf and self._buf_bytes + encoded > self._max:
            self._flush()
        self._buf.append(fragment)
        self._buf_bytes += encoded

    def finish(self) -> List[str]:
        if self._buf:
            self._flush()
        return list(self._paths)

    _REL_PATH_RE = re.compile(
        r'(?P<attr>\b(?:src|href|xlink:href)\s*=\s*)(?P<q>["\'])(?P<path>(?:content|embedded-modules)/)',
        re.IGNORECASE,
    )

    def _abs_paths(self, html: str) -> str:
        """Rewrite relative content/ and embedded-modules/ paths to /app/ absolute paths.

        Chunk files live in a subdirectory, so relative paths like content/images/...
        would resolve to chunks_ru/content/images/ instead of /app/content/images/.
        """
        def repl(m: re.Match) -> str:
            return f"{m.group('attr')}{m.group('q')}/app/{m.group('path')}"
        return self._REL_PATH_RE.sub(repl, html)

    def _flush(self) -> None:
        path = os.path.join(self._dir, f"chunk_{self._index:04d}.html")
        header = generate_html_header("Extracted documentation content", lang=self._lang)
        with open(path, "w", encoding="utf-8") as f:
            f.write(header)
            f.write("\n")
            for fragment in self._buf:
                f.write(self._abs_paths(fragment))
            f.write("</body>\n</html>\n")
        self._paths.append(path)
        self._buf = []
        self._buf_bytes = 0
        self._index += 1


def process_menu_and_extract_content(
    yaml_file_path: str,
    lang: str,
    section_filter: Optional[str] = None,
    exclude_sections: Optional[List[str]] = None,
    include_embedded_modules: bool = True,
) -> List[str]:
    """Builds chunk HTML files under chunks_{lang}/: main.yml → content/{lang}, then embedded-modules.

    Returns list of generated chunk file paths (in order).
    """
    menu_items = traverse_menu_to_list(
        yaml_file_path, lang,
        only_top_section=section_filter,
        exclude_top_sections=exclude_sections,
    )
    chunk_dir = intermediate_html_dir(lang)
    base_content = content_base_path(lang)

    writer = _ChunkWriter(chunk_dir, lang)

    for item in menu_items:
        url = item["url"]
        if not url:
            continue

        clean_url = url.lstrip("/")
        if clean_url.startswith("modules/"):
            clean_url = clean_url.replace("modules/", "embedded-modules/", 1)
        html_path = os.path.join(base_content, clean_url)

        if url.endswith("/"):
            html_path = os.path.join(html_path, "index.html")
        else:
            if not html_path.endswith(".html"):
                html_path += ".html"

        if not os.path.isfile(html_path):
            continue
        content = extract_content_from_html(html_path)
        if not content:
            continue

        soup = BeautifulSoup(content, "html.parser")
        postprocess_extracted_docs_soup(soup, lang)
        content = str(soup)
        content = fix_image_paths(content, lang=lang)
        writer.write(content)

    if include_embedded_modules:
        _append_embedded_modules_to_writer(writer, lang)

    return writer.finish()


def _append_embedded_modules_to_writer(writer: "_ChunkWriter", lang: str) -> None:
    """Append embedded-modules content to the chunk writer."""
    root = embedded_modules_root(lang)
    if not os.path.isdir(root):
        print(f"Note: embedded modules path not found, skipping: {root}")
        return

    module_slugs = sorted(
        d
        for d in os.listdir(root)
        if os.path.isdir(os.path.join(root, d))
    )
    module_jobs: List[Tuple[str, str, List[str]]] = []
    for slug in module_slugs:
        mod_path = os.path.join(root, slug)
        try:
            names = os.listdir(mod_path)
        except OSError as e:
            print(f"Warning: cannot list {mod_path}: {e}")
            continue
        html_files = sorted(
            n
            for n in names
            if n.lower().endswith(".html") and os.path.isfile(os.path.join(mod_path, n))
        )
        if not html_files:
            continue
        html_files.sort(key=embedded_module_html_sort_key)
        module_jobs.append((slug, mod_path, html_files))

    if not module_jobs:
        return

    writer.write(
        f'<h1 id="deckhouse-platform-modules-section">{MODULES_SECTION_H1[lang]}</h1>\n'
    )

    for slug, mod_path, html_files in module_jobs:
        index_path = os.path.join(mod_path, "index.html")
        module_base_title = _extract_module_base_title_from_index_path(index_path)

        for fname in html_files:
            html_path = os.path.join(mod_path, fname)
            content = extract_content_from_html(html_path)
            if not content:
                continue
            soup = BeautifulSoup(content, "html.parser")
            postprocess_extracted_docs_soup(soup, lang)
            demote_headings_one_level(soup)
            if fname.lower() != "index.html":
                demote_headings_one_level(soup)
            if fname.lower() == "index.html":
                h_mod = soup.find(
                    "h2",
                    class_=lambda c: c and "docs__title" in c,
                )
                if h_mod is not None:
                    h_mod["id"] = f"embedded-mod-{slug}"
            else:
                h_sub = soup.find(
                    "h3",
                    class_=lambda c: c and "docs__title" in c,
                )
                if h_sub is not None:
                    if module_base_title:
                        shortened = _strip_repeated_module_title_from_subpage(
                            h_sub.get_text(), module_base_title
                        )
                        if shortened is not None:
                            h_sub.clear()
                            h_sub.append(shortened)
                    page_stem = os.path.splitext(fname)[0]
                    safe_page = re.sub(r"[^a-zA-Z0-9_-]+", "-", page_stem).strip("-")
                    h_sub["id"] = f"embedded-mod-{slug}-{safe_page}"
            content = str(soup)
            content = fix_embedded_module_relative_resource_paths(content, slug, lang)
            content = fix_image_paths(
                content, apply_special_path_rewrites=False, lang=lang
            )
            writer.write(content)


def generate_cover_html(
    lang: str,
    title: str,
    doc_version: str,
    tmp_dir: str,
) -> str:
    """Writes a cover page HTML file and returns its path."""
    if lang == "ru":
        date_str = datetime.now().strftime("%d.%m.%Y")
        date_label = f"Дата генерации: {date_str}"
    else:
        date_str = datetime.now().strftime("%B %d, %Y")
        date_label = f"Generated: {date_str}"

    if doc_version:
        version_label = f"Версия {doc_version}" if lang == "ru" else f"Version {doc_version}"
        version_line = f"<p class=\"version\">{version_label}</p>"
    else:
        version_line = ""

    html = f"""<!DOCTYPE html>
<html lang="{lang}">
<head>
<meta charset="utf-8">
<style>
  @page {{ margin: 0; }}
  html, body {{
    margin: 0;
    padding: 0;
    width: 210mm;
    height: 297mm;
    font-family: DejaVu Sans, Arial, sans-serif;
  }}
  table.layout {{
    width: 210mm;
    height: 297mm;
    border-collapse: collapse;
  }}
  td.center {{
    text-align: center;
    vertical-align: middle;
    padding: 2cm 3cm;
  }}
  td.bottom {{
    text-align: center;
    vertical-align: bottom;
    padding-bottom: 2cm;
    font-size: 11pt;
    color: #444;
  }}
  h1 {{
    font-size: 24pt;
    margin: 0 0 0.4em 0;
    line-height: 1.3;
  }}
  p.version {{
    font-size: 16pt;
    margin: 0;
  }}
</style>
</head>
<body>
<table class="layout">
  <tr style="height: 85%;">
    <td class="center">
      <h1>{title}</h1>
      {version_line}
    </td>
  </tr>
  <tr style="height: 15%;">
    <td class="bottom">{date_label}</td>
  </tr>
</table>
</body>
</html>
"""
    cover_path = os.path.join(tmp_dir, f"cover_{lang}.html")
    with open(cover_path, "w", encoding="utf-8") as f:
        f.write(html)
    return cover_path


def _wkhtml_ui_strings(
    lang: str,
    doc_version: str,
    guide_title_en: str = "Administrator's guide",
    guide_title_ru: str = "Справочник администратора",
) -> Tuple[str, str, str]:
    """Header left, footer right, TOC title for wkhtmltopdf."""
    if lang == "en":
        guide_title = guide_title_en
        if doc_version:
            header_left = (
                f"Deckhouse Kubernetes Platform {doc_version}. "
                f"{guide_title} - [section]"
            )
        else:
            header_left = (
                f"Deckhouse Kubernetes Platform. "
                f"{guide_title} - [section]"
            )
        footer_right = "Page [page]"
        toc_header = "Contents"
    else:
        guide_title = guide_title_ru
        if doc_version:
            header_left = (
                f"Deckhouse Kubernetes Platform {doc_version}. "
                f"{guide_title} - [section]"
            )
        else:
            header_left = (
                f"Deckhouse Kubernetes Platform. {guide_title} - [section]"
            )
        footer_right = "Страница [page]"
        toc_header = "Содержание"
    return header_left, footer_right, toc_header


def _generate_localized_toc_xsl(toc_header: str, tmpdir: str) -> str:
    """Create a copy of toc_template.xsl with the localized TOC title baked in."""
    with open("toc_template.xsl", "r", encoding="utf-8") as f:
        xsl_content = f.read()
    xsl_content = xsl_content.replace(
        "select=\"'Contents'\"",
        f"select=\"'{toc_header}'\"",
    )
    out_path = os.path.join(tmpdir, "toc_template.xsl")
    with open(out_path, "w", encoding="utf-8") as f:
        f.write(xsl_content)
    return out_path


def _env_truthy(name: str) -> bool:
    v = os.environ.get(name, "").strip().lower()
    return v in ("1", "true", "yes", "on")


def selected_pdf_langs() -> Tuple[str, ...]:
    """BUILD_LANG=ru|en restricts build to one language; unset means both."""
    build_lang = os.environ.get("BUILD_LANG", "").strip().lower()
    if not build_lang:
        return SUPPORTED_LANGS
    if build_lang not in SUPPORTED_LANGS:
        print(
            f"Error: BUILD_LANG must be one of {SUPPORTED_LANGS}, got '{build_lang}'.",
            file=sys.stderr,
        )
        sys.exit(2)
    return (build_lang,)


# === Main execution ===
if __name__ == "__main__":
    base_pdf = os.environ.get("PDF_OUTPUT_PATH", "deckhouse-admin-guide.pdf")
    doc_version = os.environ.get("DOC_VERSION", "").strip()
    section_filter = os.environ.get("SECTION_FILTER", "").strip() or None
    _exc_raw = os.environ.get("EXCLUDE_SECTIONS", "").strip()
    exclude_sections = [s.strip() for s in _exc_raw.split(",") if s.strip()] or None
    guide_title_en = os.environ.get("GUIDE_TITLE_EN", "").strip() or "Administrator's guide"
    guide_title_ru = os.environ.get("GUIDE_TITLE_RU", "").strip() or "Справочник администратора"
    langs_to_build = selected_pdf_langs()

    _cover_tmpdir = tempfile.mkdtemp(prefix="pdf-cover-")

    for lang in langs_to_build:
        print(f"Starting HTML content extraction ({lang})...")
        chunk_paths = process_menu_and_extract_content(
            SIDEBAR_YAML,
            lang,
            section_filter=section_filter,
            exclude_sections=exclude_sections,
            include_embedded_modules=(section_filter is None),
        )
        print(f"Result: {len(chunk_paths)} chunk(s) in {intermediate_html_dir(lang)}/")

        pdf_out = pdf_output_path_for_lang(base_pdf, lang)
        print(f"Generating PDF ({lang}) → {pdf_out}...")
        header_left, footer_right, toc_header = _wkhtml_ui_strings(
            lang, doc_version, guide_title_en=guide_title_en, guide_title_ru=guide_title_ru
        )

        cover_title = (
            f"Deckhouse Kubernetes Platform {guide_title_en}"
            if lang == "en"
            else f"Deckhouse Kubernetes Platform {guide_title_ru}"
        )
        cover_path = generate_cover_html(lang, cover_title, doc_version, _cover_tmpdir)
        toc_xsl_path = _generate_localized_toc_xsl(toc_header, _cover_tmpdir)

        # Build page-object list: each chunk is a separate wkhtmltopdf page object.
        # This keeps individual HTML files small so Qt WebKit does not scale fonts down.
        page_args: List[str] = []
        for chunk_path in chunk_paths:
            page_args.extend(["page", chunk_path])

        in_ci = bool(os.environ.get("CI") or os.environ.get("GITHUB_ACTIONS"))
        wkhtmltopdf_cmd = [
            "wkhtmltopdf",
            "--page-size", "A4",
            "--margin-left", "2cm",
            "--margin-right", "2cm",
            "--margin-top", "1cm",
            "--margin-bottom", "1cm",
            "--enable-local-file-access",
            "--dpi", "96",
            "--load-error-handling", "ignore",
            "--disable-external-links",
            "--disable-javascript",
            "--outline",
            "--outline-depth", "3",
            "--header-left", header_left,
            "--header-font-size", "8",
            "--header-spacing", "3",
            "--header-line",
            "--footer-right", footer_right,
            "--footer-font-size", "8",
            "--footer-spacing", "3",
            "--footer-line",
            "cover", cover_path,
            "toc",
            "--toc-header-text", toc_header,
            "--toc-text-size-shrink", "1",
            "--toc-level-indentation", "20",
            "--xsl-style-sheet", toc_xsl_path,
            "--user-style-sheet", "toc_style.css",
            *page_args,
            pdf_out,
        ]

        try:
            proc = subprocess.run(
                wkhtmltopdf_cmd,
                check=False,
                timeout=600,
                stderr=subprocess.PIPE if in_ci else None,
            )
        except subprocess.TimeoutExpired:
            print("Error: wkhtmltopdf timed out after 10 minutes.")
            sys.exit(1)
        except FileNotFoundError:
            print("Error: wkhtmltopdf not found. Please install wkhtmltopdf.")
            sys.exit(1)

        wk_stderr = getattr(proc, "stderr", None)
        if not os.path.isfile(pdf_out):
            if wk_stderr:
                sys.stderr.buffer.write(wk_stderr)
            print(
                f"Error: PDF was not created at {pdf_out!r} "
                f"(wkhtmltopdf exit code {getattr(proc, 'returncode', 'n/a')})."
            )
            sys.exit(1)
        if proc.returncode != 0:
            if wk_stderr:
                sys.stderr.buffer.write(wk_stderr)
            print(
                f"Warning: wkhtmltopdf exited with code {proc.returncode}; \n"
                f"PDF saved to {pdf_out}."
            )
        else:
            print(f"PDF generated successfully: {pdf_out}")
