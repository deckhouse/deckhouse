#!/bin/bash

# Copyright 2021 Flant JSC
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

echo "Checking Deckhouse CLI version in the candi/version_map.yml file and in the documentation reference..."

D8_CLI_VERSION=$(yq eval .d8.d8CliVersion candi/version_map.yml)
D8_CLI_DOC_VERSION=$(jq -r .version docs/documentation/_data/reference/d8-cli.json)

echo "Deckhouse CLI version in the candi/version_map.yml is ${D8_CLI_VERSION}."
echo "Deckhouse CLI version in the documentation reference is ${D8_CLI_DOC_VERSION}."

if [ "$D8_CLI_VERSION" != "$D8_CLI_DOC_VERSION" ]; then
  echo -e "!\n! Validation failed!"
  echo "! Deckhouse CLI version in the candi/version_map.yml file (${D8_CLI_VERSION}) is different from the version in the documentation reference (${D8_CLI_DOC_VERSION})."
  echo -e "!\n! Get Deckhouse CLI ${D8_CLI_VERSION} (https://github.com/deckhouse/deckhouse-cli), and run the following command in the repository root to update the documentation reference:"
  echo -e "!\n! \td8 help-json > docs/documentation/_data/reference/d8-cli.json\n!"
  exit 1
else
  echo "Validation passed..."
fi
