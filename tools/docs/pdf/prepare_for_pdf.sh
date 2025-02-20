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

PATH_TO_PDF_PAGE="ADMIN_GUIDE.md"
PATH_TO_PDF_PAGE_RU="ADMIN_GUIDE_RU.md"
PATH_TO_PAGES='documentation/pages/'
PATH_TO_MODULES="modules"
MODULES=$(find $PATH_TO_MODULES -name "README_RU.md")
PAGES_ORDER=(
"README_RU.md"
"CONFIGURATION_RU.md"
"CR_RU.md"
"EXAMPLES_RU.md"
"FAQ_RU.md"
)

function clean () {
cat > $1 <<EOF
---
title: "Deckhouse Kubernetes Platform: $3"
permalink: $2/deckhouse-admin-guide.html
lang: $2
sidebar: none
toc: true
layout: pdf
---
EOF
}

function getname () {
  cat $1 | grep 'title: ' | sed -r 's!^[^ ]+!!' | sed -e 's/^[[:space:]0-9-]*//' | sed s/'\"'//g
}

function gettext() {
    cat $1 | sed '1,/---/ d' | sed -E "s/^#/###/g; s#(\.\./)+#./#g"
}

clean $PATH_TO_PDF_PAGE "en" "The Administrator's Guide"
clean $PATH_TO_PDF_PAGE_RU "ru" "Руководство администратора"

echo "## Deckhouse Kubernetes Platform" >> $PATH_TO_PDF_PAGE
echo "## Deckhouse Kubernetes Platform" >> $PATH_TO_PDF_PAGE_RU

echo "### Platform installation" >> $PATH_TO_PDF_PAGE
echo "### Установка платформы" >> $PATH_TO_PDF_PAGE_RU

LIST_OF_PAGES=(
"installing/README.md"
"installing/CONFIGURATION.md"
)

for ix in ${!LIST_OF_PAGES[*]}
do
  echo "Preparing page $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}"
  echo "\n## "$(getname $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}) >> $PATH_TO_PDF_PAGE
  echo "$(gettext $PATH_TO_PAGES${LIST_OF_PAGES[$ix]})" >> $PATH_TO_PDF_PAGE
  RU_PAGE="$(echo $PATH_TO_PAGES${LIST_OF_PAGES[$ix]} | sed 's/\.md$//')_RU.md"
  echo "Preparing page $RU_PAGE"
  echo "\n## "$(getname $RU_PAGE) >> $PATH_TO_PDF_PAGE_RU
  echo "$(gettext $RU_PAGE)" >> $PATH_TO_PDF_PAGE_RU
done
