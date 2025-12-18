#!/bin/sh

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

set -e

echo "Starting channels conversion..."

# Check if required environment variables are set
if [[ -z "${CHANNELS_YAML_PATH}" ]]; then
    echo "Error: CHANNELS_YAML_PATH environment variable is not set" >&2
    exit 1
fi

if [[ -z "${CHANNELS_CONF_PATH}" ]]; then
    echo "Error: CHANNELS_CONF_PATH environment variable is not set" >&2
    exit 1
fi

# Check if required tools are available
if ! command -v yq &> /dev/null; then
    echo "Error: yq is not installed" >&2
    exit 1
fi

if ! command -v jq &> /dev/null; then
    echo "Error: jq is not installed" >&2
    exit 1
fi

# Check if input file exists
if [[ ! -f "${CHANNELS_YAML_PATH}" ]]; then
    echo "Error: input file '${CHANNELS_YAML_PATH}' not found" >&2
    exit 1
fi

# Create output directory if it doesn't exist
mkdir -p "$(dirname "${CHANNELS_CONF_PATH}")"

echo "Generating ${CHANNELS_CONF_PATH} from ${CHANNELS_YAML_PATH}..."

# Convert channels.yaml to channels.conf
yq eval -o=json "${CHANNELS_YAML_PATH}" | \
jq -r '.groups[].channels[] | select(.name and .version) | if .name == "ea" then .name = "early-access" else . end | "    \"\(.name)\" \"\(.version)\";"' > "${CHANNELS_CONF_PATH}"

echo "Dump of ${CHANNELS_CONF_PATH}:"
cat "${CHANNELS_CONF_PATH}"

echo "Channels conversion completed successfully"
