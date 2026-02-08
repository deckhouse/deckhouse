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

find ${METADATA_SOURCE_DIR} -name "module.yaml" -type f | while read f; do
    dir=$(dirname "$f");
    module=$(yq -o json . "$f");

    if [ -f "$dir/oss.yaml" ]; then
        oss_data=$(yq -o json . "$dir/oss.yaml" 2>/dev/null || echo '[]')
    else
        oss_data='[]'
    fi;

    name=$(echo "$module" | jq -r '.name');
    [ "$name" != "null" ] && echo "$module" | jq --argjson oss "$oss_data" --arg name "$name" '
        if $oss != null and ($oss | type) == "array" then
            . + {oss: $oss}
        else
            .
        end |
        {$name: .}
    ';
done | jq -s '
    add as $modules |

    # Collect all OSS items with their source modules
    [
        $modules | to_entries[] | . as $module |
        $module.value.oss // [] | .[] | {
            oss_name: .name,
            module_id: $module.key,
            link: .link,
            description: .description,
            version: .version,
            logo: .logo,
            license: .license
        }
    ] |

    # Group by OSS name, take first occurrence for data
    group_by(.oss_name) |
    map({
        key: .[0].oss_name,
        value: {
            link: .[0].link,
            description: .[0].description,
            logo: .[0].logo,
            license: .[0].license,
            version: .[0].version,
            modules: [.[].module_id] | unique | sort
        }
    }) |
    sort_by(.key | ascii_downcase) |  # Case-insensitive for oss
    from_entries as $oss_map |

    {
        modules: ($modules | to_entries | sort_by(.key | ascii_downcase) | from_entries),  # Case-insensitive for raw
        oss: $oss_map
    }
' > "${METADATA_TARGET_FILE}"
