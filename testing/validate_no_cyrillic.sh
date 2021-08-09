#!/bin/bash

# Copyright 2021 Flant CJSC
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

set -Eeo pipefail

function check_cyrillic_letters() {
  if words=$(grep -Eno "[А-Яа-яЁё]+" <<< "${1}") ; then
    echo "  ERROR: Cyrillic letters found!"
    echo "${words}"
     #| jq -R '.' | jq -sc '.'
    return 1
  fi
  echo "  OK!"
}

hasCyrillicLetters=0

if [[ -n $VALIDATE_TITLE ]] ; then
  echo -n "Check title:"
  if ! check_cyrillic_letters "$VALIDATE_TITLE" ; then
    hasCyrillicLetters=1
  fi
fi

if [[ -n $VALIDATE_DESCRIPTION ]] ; then
  echo -n "Check description:"
  if ! check_cyrillic_letters "$VALIDATE_DESCRIPTION" ; then
    hasCyrillicLetters=1
  fi
fi

echo "Check new and updated lines:"

DIFF_DATA="$(git diff origin/main... --name-only -w --ignore-blank-lines --diff-filter=AM)"

if [[ -z $DIFF_DATA ]]; then
  echo "  * diff is empty"
  exit $hasCyrillicLetters
fi

for FILE_NAME in $DIFF_DATA; do
  # skip documentation
  pattern="doc-ru-.+.y[a]?ml$|_RU.md$|_ru.html$|docs/site/_.+|docs/documentation/_.+"
  if [[ "$FILE_NAME" =~ $pattern ]] ; then
    echo "  * $FILE_NAME: skip documentation"
    continue
  fi

  # skip translations
  if [[ "$FILE_NAME" == *"/i18n/"* ]] ; then
    echo "  * $FILE_NAME: skip translations"
    continue
  fi

  # skip self
  if [[ "$FILE_NAME" == *"validate_no_cyrillic.sh" ]] ; then
    echo "  * $FILE_NAME: skip self"
    continue
  fi

  # Check only new or updated lines in diff and ignore empty diffs.
  if ! FILE_DIFF=$(git diff origin/main -- $FILE_NAME | grep -v '^+++\ b' | grep '^+') ; then
    continue
  fi
  if ! check_msg=$(check_cyrillic_letters "${FILE_DIFF}" ) ; then
    echo "  * $FILE_NAME: ${check_msg}"
    hasCyrillicLetters=1
  fi
done


if [[ $hasCyrillicLetters == 0 ]]; then
  echo "Validation successful."
fi

exit $hasCyrillicLetters
