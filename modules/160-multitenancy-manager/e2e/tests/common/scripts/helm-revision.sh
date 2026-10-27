#!/usr/bin/env bash

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

# Prints the latest revision of the Helm release of a project. The controller keeps the
# releases in the d8-multitenancy-manager namespace, in Secrets labelled owner=helm and
# name=<release>; a project name within 53 characters is the release name as it is.
#
# Usage: helm-revision.sh <PROJECT_NAME>
set -Eeuo pipefail

project="${1:?project name is required}"
kubectl -n d8-multitenancy-manager get secrets -l "owner=helm,name=${project}" -o json \
  | jq -r '[.items[].metadata.labels.version | tonumber] | max // 0'
