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

{{- if or ( eq .cri "Containerd") ( eq .cri "ContainerdV2") }}
if [[ "$FIRST_BASHIBLE_RUN" == "yes" ]]; then
  CONTAINERD_PATH="$(command -v containerd 2>/dev/null || true)"
  if [[ -n "$CONTAINERD_PATH" ]]; then
    if [[ "$CONTAINERD_PATH" != "/opt/deckhouse/bin/containerd" ]]; then
    bb-log-error "containerd is detected on $HOSTNAME. Deckhouse does not support pre-provisioned containerd installations. Please uninstall containerd and try again."
    exit 1
    fi
  fi
else
  if systemctl list-unit-files containerd.service >/dev/null 2>&1; then
    if systemctl is-enabled containerd >/dev/null 2>&1 || systemctl is-active containerd >/dev/null 2>&1; then
      bb-log-error "containerd.service is enabled or running on $HOSTNAME. Deckhouse use only containerd-deckhouse.service. Please disable/stop/uninstall containerd.service to avoid further conflicts."
      exit 1
    fi
  fi
fi
{{- end }}
