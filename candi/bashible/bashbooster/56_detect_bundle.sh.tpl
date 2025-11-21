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

_bb_set_unknown_os_context() {
  bb-log-warn "Unable to determine OS from /etc/os-release, falling back to 'unknown'."
  BB_DETECTED_FAMILY="unknown"
  BB_DETECTED_BUNDLE="unknown"
  BB_DETECTED_PKG_MGR="unknown"
}

_bb_detect_os_context() {
  if [ -n "${BB_DETECTED_FAMILY:-}" ]; then
    return
  fi

  if [ ! -e /etc/os-release ]; then
    _bb_set_unknown_os_context
    return
  fi

  # shellcheck source=/dev/null
  . /etc/os-release

  case "${ID:-}" in
{{- $bashible := default (dict) .bashible }}
{{- $families := default (dict) (index $bashible "os") }}
{{- range $familyName, $family := $families }}
  {{- $pkg := $family.packageManager }}
  {{- range $distribution := $family.distributions }}
    {{ join "|" $distribution.ids }})
      BB_DETECTED_FAMILY="{{ $familyName }}"
      BB_DETECTED_BUNDLE="{{ $distribution.name }}"
      BB_DETECTED_PKG_MGR="{{ $pkg }}"
      return 0
      ;;
  {{- end }}
{{- end }}
    "")
      bb-log-warn "Can't determine OS! No ID in /etc/os-release."
      _bb_set_unknown_os_context
      return
      ;;
    *)
      bb-log-warn "${PRETTY_NAME:-Unknown OS} is not supported. Falling back to 'unknown'."
      _bb_set_unknown_os_context
      return
      ;;
  esac

  _bb_set_unknown_os_context
}

bb-is-bundle(){
  _bb_detect_os_context
  echo "$BB_DETECTED_BUNDLE"
}

bb-is-family(){
  _bb_detect_os_context
  echo "$BB_DETECTED_FAMILY"
}

bb-pkg-mgr(){
  _bb_detect_os_context
  echo "$BB_DETECTED_PKG_MGR"
}
