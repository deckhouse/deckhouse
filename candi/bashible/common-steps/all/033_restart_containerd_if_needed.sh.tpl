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

{{- if eq .cri "Containerd" }}
if bb-flag? containerd-need-restart; then
  bb-log-warning "'containerd-need-restart' flag was set, restarting containerd."
  {{- if ne .runType "ImageBuilding" }}
  if out=$(containerd config dump 2>&1); then
      systemctl restart containerd-deckhouse.service
  else
      bb-log-error "'containerd config dump' return error: $out"
      exit 1
  fi
  {{- end }}
  bb-flag-set kubelet-need-restart
  bb-flag-unset containerd-need-restart
fi
{{- end }}
