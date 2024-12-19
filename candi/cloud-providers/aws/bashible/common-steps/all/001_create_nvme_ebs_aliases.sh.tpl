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

{{- $nodeTypeList := list "CloudPermanent" }}
{{- if has .nodeGroup.nodeType $nodeTypeList }}
  {{- if eq .nodeGroup.name "master" }}

# Find and delete broken symbolic links in the /dev directory
find /dev -xtype l -delete -print

# Get the list of NVMe devices
volume_names="$(find /dev | grep -i 'nvme[0-21]n1$' || true)"

if [ ! -z "${volume_names}" ]; then
    {{- with .images.registrypackages }}
    bb-package-install "ebsnvme-id:{{ .amazonEc2Utils220 }}" "nvme-cli:{{ .nvmeCli211 }}"
    {{- end }}
    # Iterate over each found NVMe device
    for volume in ${volume_names}; do
        # Check if the found device is a symbolic link
        if [ -L "${volume}" ]; then
            echo "${volume} is a symbolic link, skipping."
            continue
        fi
        # Extract the potential symlink using 'nvme id-ctrl'
        symlink="$(/opt/deckhouse/bin/nvme id-ctrl -v "${volume}" | ( grep '^0000:' || true ) | sed -E 's/.*"(\/dev\/)?([a-z0-9]+)\.+"$/\/dev\/\2/')"
        if [ -z "${symlink}" ]; then
            symlink="$(/opt/deckhouse/bin/ebsnvme-id "${volume}" | sed -n '2p' )"
        fi

        # Correctly handle all symlink creation checks
        if [ -z "${symlink}" ]; then
            echo "Symlink for ${volume} could not be determined"
        elif [[ "${symlink}" == /dev/* ]] && [ ! -e "${symlink}" ]; then
            ln -s "${volume}" "${symlink}"
            echo "Created symlink ${symlink} -> ${volume}"
        elif [[ "${symlink}" != /dev/* ]]; then
            echo "Symlink ${symlink} does not start with /dev, skipping."
        else
            echo "Symlink ${symlink} already exists"
        fi
    done
fi

  {{- end  }}
{{- end  }}
