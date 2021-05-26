#!/bin/bash

#MODULES_SRC_DIR=/home/kar/fox/sys/antiopa/modules
#MODULES_DST_EN=/tmp/mod_en
#MODULES_DST_RU=/tmp/mod_ru

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

if [ -f modules_menu_skip ]; then
  modules_skip_list=$(cat modules_menu_skip)
fi

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
    module_name=$(echo $page | cut -d\/ -f1)
    skip=false
    for el in $modules_skip_list ; do
      if [[ $el == $module_name ]] ; then skip=true; break; fi
    done
    if [[ "$skip" == 'true' ]]; then continue; fi
    page_dst=$page
    mkdir -p $(echo "${MODULES_DST_EN}/${page_dst}" | sed -E 's|^(.+)/[^\/]+$|\1|') $(echo "${MODULES_DST_RU}/${page_dst}" | sed -E 's|^(.+)/[^\/]+$|\1|')
    if [[ -f "${absolute_path}.md" ]] && page::has_frontmatter "${absolute_path}.md"; then
        cp -f "${absolute_path}.md" "${MODULES_DST_EN}/${page_dst}.md"
    else
        cp -f "${absolute_path}_RU.md" "${MODULES_DST_EN}/${page_dst}.md"
        sed -i "1alayout: page-another-lang" "${MODULES_DST_EN}/${page_dst}.md"
        sed -i "s#lang: ru#lang: en#" "${MODULES_DST_EN}/${page_dst}.md"
        echo "INFO: No ${absolute_path}.md. Was replaced by doc from other lang."
    fi
    if [[ -f "${absolute_path}_RU.md" ]] && page::has_frontmatter "${absolute_path}_RU.md"; then
        cp -f "${absolute_path}_RU.md" "${MODULES_DST_RU}/${page_dst}.md"
        sed -i "1alang: ru" "${MODULES_DST_RU}/${page_dst}.md"
    else
        cp -f "${absolute_path}.md" "${MODULES_DST_RU}/${page_dst}.md"
        sed -i "1alayout: page-another-lang" "${MODULES_DST_RU}/${page_dst}.md"
        sed -i "1alang: ru" "${MODULES_DST_RU}/${page_dst}.md"
        echo "INFO: No ${absolute_path}_RU.md. Was replaced by doc from other lang."
    fi
done

rsync -a --exclude='*.md' --exclude-from=modules_menu_skip ${MODULES_SRC_DIR}/ ${MODULES_DST_EN}/
rsync -a --exclude='*.md' --exclude-from=modules_menu_skip ${MODULES_SRC_DIR}/ ${MODULES_DST_RU}/
