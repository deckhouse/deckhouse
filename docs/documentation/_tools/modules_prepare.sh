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

# TODO: Refactor this!

# Checks if a file has a frontmatter section.
page::has_frontmatter() {
    if [[ -f $1 ]]
    then
        head -n 1  $1 | grep -q "^---"
        if [ $? -eq 0 ]; then return 0; fi
    else
        echo "Can't find file $1" >&2
        return 1
    fi
    return 1
}

pages=$(
for i in $(find ${MODULES_SRC_DIR} -regex '.*.md' -print | sort); do
      if page::has_frontmatter "${i}"
      then
          echo $i
      else
          continue
      fi
done | sed "s|^${MODULES_SRC_DIR}/||" |  sed 's/_RU\.md/\.md/' | sed 's/\.md$//' | sort | uniq )

for page in ${pages}; do
    absolute_path="${MODULES_SRC_DIR}/${page}"
    module_original_name=$(echo $page | cut -d\/ -f1)
    module_name=$(echo $module_original_name | sed -E 's#^[0-9]+-##')

    # Skip modules, which are listed in modules_menu_skip file
    if grep -Fxq "$module_name" _tools/modules_menu_skip; then
        continue
    fi

    page_dst=$(echo $page | sed -E 's#^[0-9]+-##')
    mkdir -p $(echo "${MODULES_DST_EN}/${page_dst}" | sed -E 's|^(.+)/[^\/]+$|\1|') $(echo "${MODULES_DST_RU}/${page_dst}" | sed -E 's|^(.+)/[^\/]+$|\1|')
    if [[ -f "${absolute_path}.md" ]] && page::has_frontmatter "${absolute_path}.md"; then
        cp -f "${absolute_path}.md" "${MODULES_DST_EN}/${page_dst}.md"
    else
        cp -f "${absolute_path}_RU.md" "${MODULES_DST_EN}/${page_dst}.md"
        sed -i "1alayout: page-another-lang" "${MODULES_DST_EN}/${page_dst}.md"
        sed -i "/^lang:/{s#lang: ru#lang: en#}" "${MODULES_DST_EN}/${page_dst}.md"
        sed -Ei "/^title:/{s/title: ([\"\']?)Модуль ([-a-zA-Z0-9]+)(: .+)?([\"\']?)/title: \1The \2 module\3\4/}; /title:/{s/: настройки/: configuration/}; /title:/{s/: примеры конфигурации/: usage/}" "${MODULES_DST_EN}/${page_dst}.md"
        echo "INFO: ${absolute_path}.md is absent and has been replaced by the doc from the other lang."
    fi
    if [[ -f "${absolute_path}_RU.md" ]] && page::has_frontmatter "${absolute_path}_RU.md"; then
        cp -f "${absolute_path}_RU.md" "${MODULES_DST_RU}/${page_dst}.md"
        sed -i "1alang: ru" "${MODULES_DST_RU}/${page_dst}.md"
    else
        cp -f "${absolute_path}.md" "${MODULES_DST_RU}/${page_dst}.md"
        sed -i "1alayout: page-another-lang" "${MODULES_DST_RU}/${page_dst}.md"
        sed -i "1alang: ru" "${MODULES_DST_RU}/${page_dst}.md"
        echo "INFO: ${absolute_path}_RU.md is absent and has been replaced by the doc from the other lang."
    fi

    rsync -a --exclude='*.md' ${MODULES_SRC_DIR}/${module_original_name}/ ${MODULES_DST_EN}/${module_name}/
    rsync -a --exclude='*.md' ${MODULES_SRC_DIR}/${module_original_name}/ ${MODULES_DST_RU}/${module_name}/
done
