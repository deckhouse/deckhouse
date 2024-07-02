# Copyright 2021 Flant JSC
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

{{- if eq .nodeGroup.name "master" }}
if [[ "$FIRST_BASHIBLE_RUN" != "yes" ]]; then
  exit 0
fi

if [ -f /var/lib/bashible/kubernetes-data-device-installed ]; then
  exit 0
fi

volume_names="$(find /dev | grep -i 'nvme[0-21]n1$' || true)"
for volume in ${volume_names}
do
 symlink="$(nvme id-ctrl -v "${volume}" | ( grep '^0000:' || true ) | sed -E 's/.*"(\/dev\/)?([a-z0-9]+)\.+"$/\/dev\/\2/')"
 if [ ! -z "${symlink}" ] && [ ! -e "${symlink}" ]; then
  ln -s "${volume}" "${symlink}"
 fi
done
{{- end }}
