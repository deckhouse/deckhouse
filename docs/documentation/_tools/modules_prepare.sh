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
# TODO: Refactor this to use a more robust method of checking for frontmatter.
# E.g. better to use something like awk 'f{print} /^---/ {c++; if(c==2) exit} /^---/ {f=1}' or something like that:
# or awk
# awk 'BEGIN { in_fm = 0; has_fm = 0 }
#                     NR == 1 && /^---$/ { in_fm = 1; next }
#                     in_fm == 1 && /^---$/ {
#                     if (NR > 2) { has_fm = 1 }
#                     exit }
#                     END { exit !has_fm }' "$file")
# or
# has_frontmatter() {
#   awk 'NR==1 && $0=="---"{f=1; next} f && $0=="---"{exit 0} END{exit 1}' "$1"
# }
#
# BTW the module docs frontmatter should NOT have permalinks...

page::has_frontmatter() {
    if [[ -f $1 ]]
    then
        if awk 'NR==1 && /^---$/ { found_start=1 }
            NR>1 && /^---$/ && found_start { found_end=1; exit }
            END { exit !(found_start && found_end) }' "$1"; then
            # Has valid frontmatter
           return 0
        fi
    else
        echo "Can't find file $1" >&2
        return 1
    fi
    return 1
}

partial::prepare_page() {
    local src_file="$1"
    local dst_file="$2"
    local permalink="$3"
    local lang="$4"

    mkdir -p "$(dirname "$dst_file")"
    cp -f "$src_file" "$dst_file"

    if page::has_frontmatter "$dst_file"; then
        sed -i "1alayout: module-partial" "$dst_file"
        sed -i "1asearchable: false" "$dst_file"
        sed -i "1asitemap_include: false" "$dst_file"
        sed -i "1apermalink: ${permalink}" "$dst_file"
        if [[ "$lang" == "ru" ]]; then
            if grep -q '^lang:' "$dst_file"; then
                sed -i '/^lang:/{s#lang: .*#lang: ru#}' "$dst_file"
            else
                sed -i "1alang: ru" "$dst_file"
            fi
        fi
        return 0
    fi

    local tmp_file
    tmp_file=$(mktemp)
    {
        echo "---"
        echo "layout: module-partial"
        echo "searchable: false"
        echo "sitemap_include: false"
        echo "permalink: ${permalink}"
        if [[ "$lang" == "ru" ]]; then
            echo "lang: ru"
        fi
        echo "---"
        echo
        cat "$src_file"
    } > "$tmp_file"
    mv "$tmp_file" "$dst_file"
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

    if jq -e --arg name "$module_name" '.[] | select(. == $name)' _tools/modules_excluded.json &>/dev/null; then
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

partials=$(
for i in $(find "${MODULES_SRC_DIR}" -path '*/docs/partials/*.md' -print | sort); do
    echo "$i"
done | sed "s|^${MODULES_SRC_DIR}/||" | sort | uniq )

for partial in ${partials}; do
    module_original_name=$(echo "$partial" | cut -d\/ -f1)
    module_name=$(echo "$module_original_name" | sed -E 's#^[0-9]+-##')
    partial_rel_path=$(echo "$partial" | sed -E 's#^[^/]+/docs/partials/##')

    if [[ "$partial_rel_path" == static/* ]]; then
        continue
    fi

    partial_base_path=$(echo "$partial_rel_path" | sed -E 's/_RU\.md$/.md/' | sed -E 's/\.md$//')
    partial_dst_rel="partials/${partial_base_path}.md"

    en_src="${MODULES_SRC_DIR}/${module_original_name}/docs/partials/${partial_base_path}.md"
    ru_src="${MODULES_SRC_DIR}/${module_original_name}/docs/partials/${partial_base_path}_RU.md"

    if [[ -f "$en_src" ]]; then
        partial::prepare_page \
            "$en_src" \
            "${MODULES_DST_EN}/${module_name}/${partial_dst_rel}" \
            "en/modules/${module_name}/partials/${partial_base_path}.html" \
            "en"
    fi

    if [[ -f "$ru_src" ]]; then
        partial::prepare_page \
            "$ru_src" \
            "${MODULES_DST_RU}/${module_name}/${partial_dst_rel}" \
            "ru/modules/${module_name}/partials/${partial_base_path}.html" \
            "ru"
    fi
done
