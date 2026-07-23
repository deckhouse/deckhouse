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

# This script outputs alphabetically sorted modules list including path and revision in the YAML format.
# Example:
# ...
# modules:
#   network-gateway:
#     path: modules/network-gateway/
#     editionMinimumAvailable: ee
#   network-policy-engine:
#     path: modules/network-policy-engine/
#     ededitionMinimumAvailableition: ce
#
set -eu

MODULES_DIR="${MODULES_DIR:-/src}"

# Iterates over all module documentation files found in the specified modules directory,
# extracts the module name and its edition, and constructs a JSON object using jq.
# Each line produced by sed has the form "edition/module-name" (e.g. "ce/admission-policy-engine").
find "${MODULES_DIR}" -regex '.*/docs/README.md' -print |
  grep -E '.+/modules/[^/]+' |
  sed -E "s#^${MODULES_DIR}(.*/modules/[^/]+)/.+\$#\1#; s#^/modules/#/ce/modules/#; s#^/ee/(be/|se/|se-plus/|fe/)#/\1#;
          s#/([^/]+)/modules/([0-9]+-)?(.+)#\1/\3#" |
  sort -t/ -k 2 |
  jq -Rn --slurpfile exclude _tools/modules_excluded.json '
    reduce inputs as $line ({};
      ($line | split("/")) as $parts |
      . + {($parts[1]): {"path": ("modules/" + $parts[1] + "/"), "editionMinimumAvailable": $parts[0]}}
    ) |
    with_entries(select(.key | . as $k | $exclude[0] | index($k) | not))
  '
