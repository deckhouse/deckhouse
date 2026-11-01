#!/bin/sh
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

# Removes what an interrupted run of the CPU starvation scenarios left in the cluster of the current
# kubectl context: the labelled test namespaces with the burner and the load, the users, the RBAC
# objects, the changed Dex pods, and the updateMode Off of the VPA dex. Run it with `task cleanup`
# only when no scenario is running.

set -eu
# shellcheck source-path=SCRIPTDIR source=lib.sh
. "$(dirname "$0")/lib.sh"
cleanup_leftovers
