#!/usr/bin/python3
# -*- coding: utf-8 -*-

# Copyright 2024 Flant JSC
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

import re
import sys
import yaml
from pathlib import Path

def clean_html(text):
    text = re.sub(r'(\<(/?[^>]+)>)', ' ', text)
    return text

def clean_liquid(text):
    text = re.sub(r'(\{{(/?[^}}]+)}})', ' ', text)
    text = re.sub(r'(\{%(/?[^%}]+)%})', ' ', text)
    return text

def clean_scripts(text):
    text = re.sub(r'<script>[\s\S]+?<\/script>', ' ', text)
    text = re.sub(r'<script [\s\S]+?>[\s\S]+?<\/script>', ' ', text)
    return text

def clean_preamble(text):
    text = re.sub(r'---[\s\S]+?---', ' ', text)
    return text

def delete_code_blocks(text):
    text = re.sub(r'```[\s\S]+?```', ' ', text)
    text = re.sub(r'`[\s\S]+?`', ' ', text)
    text = re.sub(r'<code>[\s\S]+?<\/code>', ' ', text)
    text = re.sub(r'<code [\s\S]+?>[\s\S]+?<\/code>', ' ', text)
    return text

def delete_md_links(text):
    text = re.sub(r'(?:__|\[*#])|\[(.*?)\]\(.*?\)', ' ', text)
    return text

def delete_nbsp(text):
    text = text.replace('nbsp', ' ')
    return text

def find_all_keys(input_dict: dict) -> list:
    result = []
    for key, val in input_dict.items():
        if key.startswith('description'):
            result.append(val)
        if isinstance(val, dict):
            result.extend(find_all_keys(val))
    return result

if len (sys.argv) > 1:
  if sys.argv[1] == '-':
    text = sys.stdin.read()
    text = clean_preamble(text)
    text = delete_code_blocks(text)
    text = delete_md_links(text)
    text = clean_scripts(text)
    text = clean_html(text)
    text = clean_liquid(text)
    text = delete_nbsp(text)
    print(text)
  else:
    file_extension = sys.argv[1].split('.')[-1]
    if file_extension == 'html' or file_extension == 'md' or file_extension == 'liquid':
        with open(sys.argv[1], 'r') as f:
            text = f.read()
            text = clean_preamble(text)
            text = delete_code_blocks(text)
            text = delete_md_links(text)
            text = clean_scripts(text)
            text = clean_html(text)
            text = clean_liquid(text)
            text = delete_nbsp(text)
            print(text)
    elif file_extension == 'yml' or file_extension == 'yaml':
        if 'openapi' in sys.argv[1]:
            with open(sys.argv[1], 'r') as f:
                data = yaml.safe_load(Path(sys.argv[1]).read_text())
                descriptions = find_all_keys(data)
                if bool(descriptions):
                    for item in descriptions:
                        text = delete_code_blocks(str(item))
                        text = delete_md_links(text)
                        print(text)
