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

PATH_TO_PDF_PAGE="docs/documentation/pages/pdf/ADMIN_GUIDE_RU.md"
PATH_TO_PAGES='docs/documentation/pages/'
LIST_OF_PAGES=(
"installing/README_RU.md"
"DECKHOUSE_CONFIGURE_RU.md"
"DECKHOUSE_CONFIGURE_GLOBAL_RU.md"
"installing/UNINSTALL_RU.md"
"DECKHOUSE-RELEASE-CHANNELS_RU.md"
"SUPPORTED_VERSIONS_RU.md"
"SECURITY_SOFTWARE_SETUP_RU.md"
"NETWORK_SECURITY_SETUP_RU.md"
"DECKHOUSE-FAQ_RU.md"
)
PATH_TO_MODULES="modules"
MODULES=$(find $PATH_TO_MODULES -name "README_RU.md")

function clean () {
cat > $1 <<EOF
---
title: "Deckhouse Kubernetes Platform: Руководство администратора"
permalink: ru/deckhouse-admin-guide.html
lang: ru
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
    cat $1 | sed '1,/---/ d' | sed -E "s/^#/##/g; s#(\.\./)+#./#g"
}

clean $PATH_TO_PDF_PAGE

for ix in ${!LIST_OF_PAGES[*]}
do
  echo -e "\n## "$(getname $PATH_TO_PAGES${LIST_OF_PAGES[$ix]}) >> $PATH_TO_PDF_PAGE
  echo "$(gettext $PATH_TO_PAGES${LIST_OF_PAGES[$ix]})" >> $PATH_TO_PDF_PAGE
done

for file in $(find . -name "README_RU.md" | sort -t '-' -k2)
do
  if [[ $file != *"internal"* ]] && [[ $file != *"descheduler"* ]] && [[ $file != *"fe"* ]]; then
    echo -e "\n## "$(getname $file) >> $PATH_TO_PDF_PAGE
    echo "$(gettext $file)" >> $PATH_TO_PDF_PAGE
  fi
done
