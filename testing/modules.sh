#!/bin/bash

# Copyright 2023 Flant JSC
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
shopt -s failglob

# This script starts modules tests for Deckhouse.
#
# Usage: [FOCUS=<value>] [TESTS_TIMEOUT=<timeout>] modules.sh

function execute_linker() {
  WORKING_DIR=$(pwd)
  cd /deckhouse/tools/edition_linker
  go run . "$1"
  cd "$WORKING_DIR"
}

execute_linker merge

if [[ -z "${FOCUS}" ]]; then
  echo "No focus"
  TESTS_PATH="./modules/... ./global-hooks/..."
else
  echo "Focus on \"${FOCUS}\" module"
  TESTS_PATH="./$(find modules/ -type d -name "*-$FOCUS")/..."
fi

set +e
# go test doesn't like double quoting, so disable warning
# shellcheck disable=SC2086
go test ${TESTS_TIMEOUT:+"-timeout $TESTS_TIMEOUT"} -vet=off ${TESTS_PATH}
TEST_RESULT=$?
set -e

execute_linker restore
exit $TEST_RESULT
