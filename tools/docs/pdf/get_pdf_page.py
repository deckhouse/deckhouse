#!/usr/bin/python3
# -*- coding: utf-8 -*-

# Copyright 2025 Flant JSC
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

import yaml
import os
import re
import subprocess
import sys
from typing import Any, List, Dict, Optional, Tuple
from bs4 import BeautifulSoup

# Languages to build (order = PDF generation order).
SUPPORTED_LANGS: Tuple[str, ...] = ("ru", "en")

SIDEBAR_YAML = "main.yml"

# H1 before embedded-modules block in merged HTML.
MODULES_SECTION_H1: Dict[str, str] = {
    "ru": "Модули Deckhouse Kubernetes Platform",
    "en": "Deckhouse Kubernetes Platform modules",
}


def intermediate_html_path(lang: str) -> str:
    return f"extracted_content_{lang}.html"


def pdf_output_path_for_lang(base_pdf_path: str, lang: str) -> str:
    base, ext = os.path.splitext(base_pdf_path)
    if not ext:
        ext = ".pdf"
    return f"{base}_{lang}{ext}"


def content_base_path(lang: str) -> str:
    return os.path.join("content", lang)


def embedded_modules_root(lang: str) -> str:
    return os.path.join("embedded-modules", lang, "modules")


def traverse_menu_to_list(yaml_file_path: str, lang: str) -> List[Dict[str, Optional[str]]]:
    """Collects sidebar entries with localized title (lang, fallback ru/en, or plain string)."""
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

    def walk(node: Any, level: int) -> None:
        if isinstance(node, list):
            for item in node:
                walk(item, level)
        elif isinstance(node, dict):
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
                walk(folders, level + 1)

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
            font-size: 200%;
        }
        a, a:link, a:visited, a:hover, a:active {
            color: black;
            text-decoration: none;
        }
        /* Таблицы: не шире области печати, перенос длинного текста в ячейках */
        table {
            border-collapse: collapse;
            border: 1px solid black;
            max-width: 100% !important;
            width: 100% !important;
            table-layout: fixed;
            box-sizing: border-box;
        }
        table th,
        table td {
            border: 1px solid black;
            padding: 8px;
            word-wrap: break-word;
            overflow-wrap: break-word;
            word-break: break-word;
            vertical-align: top;
            box-sizing: border-box;
        }
        table th {
            background-color: #f0f0f0;
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
        /* Блоки кода (Rouge/Jekyll): длинные строки переносим, не уезжают за край PDF */
        pre,
        pre.highlight,
        .highlight pre {
            white-space: pre-wrap !important;
            word-wrap: break-word;
            overflow-wrap: break-word;
            word-break: break-word;
            max-width: 100% !important;
            box-sizing: border-box;
        }
        pre code,
        .highlight code {
            white-space: pre-wrap !important;
            word-break: break-word;
        }
        div.highlighter-rouge,
        div.highlight,
        div[class*="language-"] {
            max-width: 100%;
            box-sizing: border-box;
        }
        /* Инлайн-код и длинные токены в абзаце */
        code,
        code.highlighter-rouge,
        code[class*="language-"] {
            overflow-wrap: break-word;
            word-break: break-word;
            max-width: 100%;
            box-sizing: border-box;
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
            return content_div.prettify()
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

    tabs_divs = soup.find_all("div", class_="tabs")
    for tabs_div in tabs_divs:
        parent = tabs_div.find_parent()
        if not parent:
            continue

        content_divs = parent.find_all("div", class_=lambda x: x and "tabs__content" in x)

        active_content_div = None
        active_id = None

        for content_div in content_divs:
            attrs_cd = getattr(content_div, "attrs", None)
            if not isinstance(attrs_cd, dict):
                continue
            classes = attrs_cd.get("class", [])
            if not isinstance(classes, list):
                classes = [classes] if classes else []
            if "active" in classes or "activ" in classes:
                active_content_div = content_div
                active_id = attrs_cd.get("id")
                break

        if not active_id:
            continue

        for content_div in content_divs:
            attrs_cd = getattr(content_div, "attrs", None)
            cid = attrs_cd.get("id") if isinstance(attrs_cd, dict) else None
            if cid != active_id:
                content_div.decompose()

        tab_links = tabs_div.find_all("a", class_=lambda x: x and "tabs__btn" in x)
        for link in tab_links:
            attrs_link = getattr(link, "attrs", None)
            if not isinstance(attrs_link, dict):
                continue
            onclick = str(attrs_link.get("onclick") or "")
            matches = re.findall(r"['\"]([^'\"]+)['\"]", onclick)
            if matches:
                link_id = matches[-1]
                if link_id != active_id:
                    link.decompose()
            else:
                classes = attrs_link.get("class", [])
                if not isinstance(classes, list):
                    classes = [classes] if classes else []
                if "active" not in classes and "activ" not in classes:
                    link.decompose()

    if lang == "en":
        remove_sections_with_exact_heading(soup, "External components")
    else:
        remove_sections_with_exact_heading(soup, "Внешние компоненты")


def append_embedded_modules(out_f, lang: str) -> None:
    """After main.yml/content: append embedded-modules/{lang}/modules/* in module order, pages sorted."""
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

    out_f.write(
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
            # index: заголовок модуля = h2 под общим h1 «Модули…». Остальные страницы — ещё на уровень ниже (h3+), вложенные в модуль в оглавлении PDF.
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
            out_f.write(content)


def process_menu_and_extract_content(yaml_file_path: str, lang: str) -> None:
    """Builds extracted_content_{lang}.html: main.yml → content/{lang}, then embedded-modules."""
    menu_items = traverse_menu_to_list(yaml_file_path, lang)
    intermediate = intermediate_html_path(lang)
    base_content = content_base_path(lang)

    with open(intermediate, "w", encoding="utf-8") as out_f:
        header = generate_html_header("Extracted documentation content", lang=lang)
        out_f.write(header)
        out_f.write("\n")
        for item in menu_items:
            url = item["url"]
            if not url:
                continue  # Skip elements without URL

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
            if content is None:
                continue

            if content:
                soup = BeautifulSoup(content, "html.parser")
                postprocess_extracted_docs_soup(soup, lang)
                content = str(soup)
                content = fix_image_paths(content, lang=lang)

            out_f.write(content)

        append_embedded_modules(out_f, lang)

        out_f.write("</body>\n</html>\n")


def _wkhtml_ui_strings(lang: str, dkp_doc_version: str) -> Tuple[str, str, str]:
    """Header left, footer right, TOC title for wkhtmltopdf."""
    if lang == "en":
        if dkp_doc_version:
            header_left = (
                f"Deckhouse Kubernetes Platform {dkp_doc_version}. "
                "Administrator's guide - [section]"
            )
        else:
            header_left = (
                "Deckhouse Kubernetes Platform. "
                "Administrator's guide - [section]"
            )
        footer_right = "Page [page]"
        toc_header = "Contents"
    else:
        if dkp_doc_version:
            header_left = (
                f"Deckhouse Kubernetes Platform {dkp_doc_version}. "
                "Справочник администратора - [section]"
            )
        else:
            header_left = (
                "Deckhouse Kubernetes Platform. Справочник администратора - [section]"
            )
        footer_right = "Страница [page]"
        toc_header = "Содержание"
    return header_left, footer_right, toc_header


def _env_truthy(name: str) -> bool:
    v = os.environ.get(name, "").strip().lower()
    return v in ("1", "true", "yes", "on")


def selected_pdf_langs() -> Tuple[str, ...]:
    """ONLY_RU=1 or ONLY_EN=1 (via env) restricts build to one language; both set → error."""
    only_ru = _env_truthy("ONLY_RU")
    only_en = _env_truthy("ONLY_EN")
    if only_ru and only_en:
        print(
            "Error: use only one of ONLY_RU=1 or ONLY_EN=1, not both.",
            file=sys.stderr,
        )
        sys.exit(2)
    if only_ru:
        return ("ru",)
    if only_en:
        return ("en",)
    return SUPPORTED_LANGS


# === Main execution ===
if __name__ == "__main__":
    base_pdf = os.environ.get("PDF_OUTPUT_PATH", "deckhouse-admin-guide.pdf")
    dkp_doc_version = os.environ.get("DKP_DOC_VERSION", "").strip()
    langs_to_build = selected_pdf_langs()

    for lang in langs_to_build:
        print(f"Starting HTML content extraction ({lang})...")
        process_menu_and_extract_content(SIDEBAR_YAML, lang)
        print(f"Result saved to: {intermediate_html_path(lang)}")

        pdf_out = pdf_output_path_for_lang(base_pdf, lang)
        print(f"Generating PDF ({lang}) → {pdf_out}...")
        header_left, footer_right, toc_header = _wkhtml_ui_strings(
            lang, dkp_doc_version
        )

        wkhtmltopdf_cmd = [
            "wkhtmltopdf",
            "--page-size", "A4",
            "--margin-left", "2cm",
            "--margin-right", "2cm",
            "--margin-top", "1cm",
            "--margin-bottom", "1cm",
            "--enable-local-file-access",
            "--dpi", "96",
            "--minimum-font-size", "8",
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
            "toc",
            "--toc-header-text", toc_header,
            "--toc-text-size-shrink", "1",
            "--toc-level-indentation", "20",
            "--xsl-style-sheet", "toc_template.xsl",
            "--user-style-sheet", "toc_style.css",
            intermediate_html_path(lang),
            pdf_out,
        ]

        try:
            proc = subprocess.run(
                wkhtmltopdf_cmd,
                check=False,
                timeout=600,
            )
        except subprocess.TimeoutExpired:
            print("Error: wkhtmltopdf timed out after 10 minutes.")
            sys.exit(1)
        except FileNotFoundError:
            print("Error: wkhtmltopdf not found. Please install wkhtmltopdf.")
            sys.exit(1)

        if not os.path.isfile(pdf_out):
            print(
                f"Error: PDF was not created at {pdf_out!r} "
                f"(wkhtmltopdf exit code {getattr(proc, 'returncode', 'n/a')})."
            )
            sys.exit(1)
        if proc.returncode != 0:
            print(
                f"Warning: wkhtmltopdf exited with code {proc.returncode}; \n"
                f"PDF saved to {pdf_out}."
            )
        else:
            print(f"PDF generated successfully: {pdf_out}")
