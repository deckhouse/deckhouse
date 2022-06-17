#!/bin/bash

# Copyright 2022 Flant JSC
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

STATUS=0

printf '
######################################################################################################################
###
###                   Markdown linter report

'

make lint-markdown 1>/dev/null
EXIT_CODE=$?
if [ $EXIT_CODE -ne "0" ]; then
   printf '
To run linter locally execute the following command in the Deckhouse repo:
   make lint-markdown

To run linter locally and AUTOMATICALLY FIX basic problems execute the following command in the Deckhouse repo:
   make lint-markdown-fix

'
   STATUS=$EXIT_CODE
else
   echo 'All checks passed.'
fi

printf '
###                   Powered by https://github.com/DavidAnson/markdownlint/
######################################################################################################################

'

exit $STATUS
