#!/usr/bin/env bash
{{- /*
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
*/}}

mkdir -p /var/lib/bashible

{{- $bbnn := .Files.Get "deckhouse/candi/bashible/bb_node_name.sh.tpl" }}
{{ tpl (printf "%s\n{{- template \"bb-discover-node-name\" . }}" $bbnn) . | nindent 0 }}

bb-discover-node-name

{{- $bbni := .Files.Get "deckhouse/candi/bashible/bb_node_ip.sh.tpl" }}
{{- tpl ( $bbni ) .  | nindent 0 }}
