#!/bin/bash

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

cat > docs/documentation/pages/DESCRIPTIONS_TEST_RU.md <<EOF
---
title: "Тест Descriptions"
permalink: ru/deckhouse-test-descriptions.html
lang: ru
layout: none
---

{% for page in site.pages %}

{% unless page.description %}
* {{ page.title }} — {{ page.permalink }} <br>
  {% if page.name == "CONFIGURATION.md" or page.name == "CONFIGURATION_RU.md" %}
  {%- assign moduleName = page['module-kebab-name'] %}
  {%- assign description = site.data.i18n.common.description_configuration[page.lang] | replace: '<MODULENAME>', moduleName %}
  {% elsif page.name == "CR.md" or page.name == "CR_RU.md" %}
  {%- assign moduleName = page['module-kebab-name'] %}
  {%- assign description = site.data.i18n.common.description_cr[page.lang] | replace: '<MODULENAME>', moduleName %}
  {% else %}
  {%- assign description = page.content | markdownify | strip_html | normalizeSearchContent | strip_newlines | strip | truncate: 200 %}
  {% endif %}
  DESCRIPTION: {{ description }}
  {% endunless %}

{% endfor %}
EOF
