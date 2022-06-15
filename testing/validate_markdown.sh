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

docker run --rm -v $PWD:/workdir --entrypoint sh ghcr.io/igorshubovych/markdownlint-cli@sha256:2e22b4979347f70e0768e3fef1a459578b75d7966e4b1a6500712b05c5139476 -c \
 "echo
  echo '######################################################################################################################'
  echo '###'
  echo '###                   Markdown linter report'
  echo
  STATUS=0
  markdownlint --config testing/markdownlint.yaml -p testing/.markdownlintignore \"**/*.md\"
  EXIT_CODE=\$?
  if [ \$EXIT_CODE -ne "0" ]; then
     echo
     echo 'To run linter locally execute the following command in the Deckhouse repo:'
     echo 'docker run --rm -ti -v \$PWD:/workdir ghcr.io/igorshubovych/markdownlint-cli@sha256:2e22b4979347f70e0768e3fef1a459578b75d7966e4b1a6500712b05c5139476 --config testing/markdownlint.yaml -p testing/.markdownlintignore \"**/*.md\"'
     echo
     echo 'To run linter locally and AUTOMATICALLY FIX basic problems execute the following command in the Deckhouse repo:'
     echo 'docker run --rm -ti -v \$PWD:/workdir ghcr.io/igorshubovych/markdownlint-cli@sha256:2e22b4979347f70e0768e3fef1a459578b75d7966e4b1a6500712b05c5139476 --config testing/markdownlint.yaml -p testing/.markdownlintignore --fix \"**/*.md\"'
     STATUS=\$EXIT_CODE
  else
     echo 'All checks passed.'
  fi
  echo
  echo '###                   Powered by https://github.com/DavidAnson/markdownlint/'
  echo '######################################################################################################################'
  echo
  exit \$STATUS
  "
