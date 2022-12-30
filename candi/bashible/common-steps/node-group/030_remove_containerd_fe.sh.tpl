# Copyright 2022 Flant JSC
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
# TODO remove after 1.43 release !!!
{{- if eq .cri "Containerd" }}
# Remove containerd-flant-edition if installed
if [[ -f /usr/local/bin/containerd ]]; then
  rm -f /usr/local/bin/containerd
fi

if [[ -d /var/cache/registrypackages/containerd-flant-edition ]]; then
  rm -rf /var/cache/registrypackages/containerd-flant-edition
fi

if [[ -f /etc/systemd/system/containerd.service.d/override.conf ]]; then
  rm -rf /etc/systemd/system/containerd.service.d
  systemctl daemon-reload
  bb-flag-set containerd-need-restart
fi
{{- end }}
