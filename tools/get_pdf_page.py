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
from typing import Any, List, Dict, Optional, Tuple
from bs4 import BeautifulSoup

# === Global variables ===
BASE_CONTENT_PATH = "content"  # ← REPLACE WITH YOUR PATH!
INTERMEDIATE_HTML_FILE = "extracted_content.html"      # Intermediate HTML file name
SIDEBAR_YAML = "main.yml"  # ← specify path to YAML file

def traverse_menu_ru_to_list(yaml_file_path: str) -> List[Dict[str, Optional[str]]]:
    """Collects all elements with title.ru into a list of dictionaries."""
    with open(yaml_file_path, "r", encoding="utf-8") as f:
        data = yaml.safe_load(f)

    results: List[Dict[str, Optional[str]]] = []

    def get_ru_title(obj: Any) -> str:
        if isinstance(obj, dict):
            title = obj.get("title")
            if isinstance(title, dict):
                return title.get("ru", "")
            elif isinstance(title, str):
                return title
        return ""

    def walk(node: Any, level: int) -> None:
        if isinstance(node, list):
            for item in node:
                walk(item, level)
        elif isinstance(node, dict):
            title_ru = get_ru_title(node)
            if title_ru:
                url = node.get("url")
                results.append({
                    "title": title_ru,
                    "level": level,
                    "url": url
                })
            folders = node.get("folders")
            if folders is not None:
                walk(folders, level + 1)

    entries = data.get("entries", [])
    walk(entries, level=0)
    return results


def generate_html_header(title: str = "Extracted content") -> str:
    """Generates HTML header."""
    html = """<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="utf-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>""" + title + """</title>
    <style>
        body {
            font-size: 200%;
        }
        table {
            border-collapse: collapse;
            border: 1px solid black;
        }
        table th,
        table td {
            border: 1px solid black;
            padding: 8px;
        }
        table th {
            background-color: #f0f0f0;
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


def fix_image_paths(content: str) -> str:
    """Fixes image paths in HTML content."""
    if not content:
        return content
    
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
    
    # First, fix special paths
    # Process only paths that are not yet in the correct subdirectory
    for img_name, subdir in special_paths.items():
        escaped_img_name = re.escape(img_name)
        escaped_subdir = re.escape(subdir)
        # Look for paths like /images/IMG_NAME or images/IMG_NAME or content/images/IMG_NAME
        # But skip paths that already contain the correct subdirectory
        # Pattern: find paths that do NOT contain content/images/{subdir}/ before the filename
        # Account for possible file extensions (.png, .svg, .jpg, .ru.png, etc.)
        pattern = r'(?<!content/images/' + escaped_subdir + r'/)(content/images/|images/|/images/)' + escaped_img_name + r'(\.ru)?(\.png|\.svg|\.jpg|\.jpeg|\.gif)'
        replacement = f'content/images/{subdir}/{img_name}\\2\\3'
        content = re.sub(pattern, replacement, content)
    
    # Then fix general paths: /images/ -> content/images/
    content = re.sub(r'src="/images/', 'src="content/images/', content)
    content = re.sub(r'xlink:href="/images/', 'xlink:href="content/images/', content)
    content = re.sub(r'href="/images/', 'href="content/images/', content)
    
    # Fix relative paths: images/ or ./images/ -> content/images/
    # But don't touch paths that already start with content/images/
    content = re.sub(r'src="(?<!content/)images/', 'src="content/images/', content)
    content = re.sub(r'src="\./images/', 'src="content/images/', content)
    content = re.sub(r'xlink:href="(?<!content/)images/', 'xlink:href="content/images/', content)
    content = re.sub(r'xlink:href="\./images/', 'xlink:href="content/images/', content)
    content = re.sub(r'href="(?<!content/)images/', 'href="content/images/', content)
    content = re.sub(r'href="\./images/', 'href="content/images/', content)
    
    # Fix single quotes
    content = re.sub(r"src='/images/", "src='content/images/", content)
    content = re.sub(r"xlink:href='/images/", "xlink:href='content/images/", content)
    content = re.sub(r"href='/images/", "href='content/images/", content)
    content = re.sub(r"src='(?<!content/)images/", "src='content/images/", content)
    content = re.sub(r"src='\./images/", "src='content/images/", content)
    content = re.sub(r"xlink:href='(?<!content/)images/", "xlink:href='content/images/", content)
    content = re.sub(r"xlink:href='\./images/", "xlink:href='content/images/", content)
    content = re.sub(r"href='(?<!content/)images/", "href='content/images/", content)
    content = re.sub(r"href='\./images/", "href='content/images/", content)
    
    # Fix erroneous paths .content/images/ -> content/images/
    content = re.sub(r'\.content/images/', 'content/images/', content)
    
    # Fix paths for stronghold files: remove .ru from name if file doesn't exist
    # Files are named admin-guide-image*.png, not admin-guide-image*.ru.png
    content = re.sub(r'content/images/stronghold/(admin-guide-image\d+)\.ru\.png', r'content/images/stronghold/\1.png', content)
    # Also fix image2.ru.png -> image2.png (if image2.ru.png file doesn't exist)
    content = re.sub(r'content/images/stronghold/image2\.ru\.png', r'content/images/stronghold/image2.png', content)
    
    return content


def process_menu_and_extract_content(yaml_file_path: str) -> None:
    """Main function: traverses menu, extracts content, writes to file."""
    menu_items = traverse_menu_ru_to_list(yaml_file_path)

    with open(INTERMEDIATE_HTML_FILE, "w", encoding="utf-8") as out_f:
        # Write HTML header with all assets
        header = generate_html_header("Extracted documentation content")
        out_f.write(header)
        out_f.write("\n")
        for item in menu_items:
            url = item["url"]
            if not url:
                continue  # Skip elements without URL

            # Convert URL to file path
            # Remove leading slash if present
            clean_url = url.lstrip("/")
            # If URL starts with "modules/", replace with "embedded-modules/"
            if clean_url.startswith("modules/"):
                clean_url = clean_url.replace("modules/", "embedded-modules/", 1)
            html_path = os.path.join(BASE_CONTENT_PATH, clean_url)

            # Handle index files: if path ends with '/', look for index.html
            if url.endswith("/"):
                html_path = os.path.join(html_path, "index.html")
            else:
                # Make sure file has .html extension
                if not html_path.endswith(".html"):
                    html_path += ".html"

            content = extract_content_from_html(html_path)
            if content is None:
                content = ""
            
            # Remove div blocks with page-navigation class
            if content:
                soup = BeautifulSoup(content, "html.parser")
                page_nav_divs = soup.find_all("div", class_="page-navigation")
                for div in page_nav_divs:
                    div.decompose()
                
                # Remove div blocks with info alert__wrap class containing specified text
                info_alert_divs = soup.find_all("div", class_="info alert__wrap")
                for div in info_alert_divs:
                    # Check if div contains paragraph with target text
                    paragraphs = div.find_all("p")
                    for p in paragraphs:
                        # Normalize whitespace for comparison
                        p_text = ' '.join(p.get_text().split())
                        target_text = 'Ссылки на связанную документацию и API:'
                        if target_text in p_text:
                            div.decompose()
                            break
                
                content = str(soup)
                
                # Fix image paths
                content = fix_image_paths(content)

            # Write to file
            out_f.write(content)
        
        # Close HTML tags
        out_f.write("</body>\n</html>\n")


# === Main execution ===
if __name__ == "__main__":
    print("Starting HTML content extraction...")
    process_menu_and_extract_content(SIDEBAR_YAML)
    print(f"Result saved to: {INTERMEDIATE_HTML_FILE}")
    
    # Generate PDF using wkhtmltopdf
    print("Generating PDF...")
    wkhtmltopdf_cmd = [
        "wkhtmltopdf",
        "--page-size", "A4",
        "--margin-left", "2cm",
        "--margin-right", "2cm",
        "--margin-top", "1cm",
        "--margin-bottom", "1cm",
        "--enable-local-file-access",
        "--outline",
        "--outline-depth", "3",
        "--header-left", "Deckhouse Kubernetes Platform CSE. Справочник администратора - [section]",
        "--header-font-size", "8",
        "--header-spacing", "3",
        "--header-line",
        "--footer-right", "Страница [page]",
        "--footer-font-size", "8",
        "--footer-spacing", "3",
        "--footer-line",
        "toc",
        "--toc-header-text", "Deckhouse Kubernetes Platform CSE: Справочник администратора",
        "--toc-text-size-shrink", "1",
        "--toc-level-indentation", "20",
        INTERMEDIATE_HTML_FILE,
        "deckhouse-admin-guide.pdf"
    ]
    
    try:
        result = subprocess.run(wkhtmltopdf_cmd, check=True, capture_output=True, text=True)
        print("PDF generated successfully: deckhouse-admin-guide.pdf")
    except subprocess.CalledProcessError as e:
        print(f"Error generating PDF: {e}")
        print(f"stderr: {e.stderr}")
        exit(1)
    except FileNotFoundError:
        print("Error: wkhtmltopdf not found. Please install wkhtmltopdf.")
        exit(1)