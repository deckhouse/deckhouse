#!/bin/bash

# Copyright 2026 Flant JSC
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

# No-op stub.
#
# The Validations workflow runs on pull_request_target, so GitHub takes it from
# the default branch while the checkout is this branch. main calls this script
# to verify that tools/docs/spelling/wordlist and dictionaries/dev_OPS.dic stay
# in sync; the check landed after 1.73 was cut, so every pull request into this
# branch died here with exit 127 over a validation the branch never carried.
#
# Keeping an executable stub lets the step succeed without pretending the check
# ran. Replace it with the real script from main if the check is backported.

set -Eeo pipefail

echo "::notice::validate_dictionary_sync.sh is a no-op stub in this branch, the wordlist <-> dev_OPS.dic sync is not verified here"

exit 0
