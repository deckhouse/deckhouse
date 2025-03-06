#!/usr/bin/env bash
{{- /*
# Copyright 2025 Flant JSC
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
*/}}
    
uid="$(id -u deckhouse 2>/dev/null || true)"
gid="$(getent group deckhouse | cut -d: -f3 || true)"

if [ -n "$uid" ] || [ -n "$gid" ]; then
    echo "deckhouse user or group already exists with id: uid=${uid}, gid=${gid}"
    exit 1
fi