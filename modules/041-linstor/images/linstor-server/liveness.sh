#!/bin/sh

# Copyright 2023 Flant JSC
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

# Sometimes nodes can be shown as Online without established connection to them.
# This is a workaround for https://github.com/LINBIT/linstor-server/issues/331

# Collect list of satellite nodes
SATELLITES_ONLINE=$(linstor -m --output-version=v1 n l | jq -r '.[][] | select(.type == "SATELLITE" and .connection_status == "ONLINE").name' || true)
if [ -z "$SATELLITES_ONLINE" ]; then
  exit 0
fi

# Check online nodes with lost connection
linstor -m --output-version=v1 sp l -s DfltDisklessStorPool -n $SATELLITES_ONLINE | jq '.[][].reports[]?.message' | grep 'No active connection to satellite'
if [ $? -eq 0 ]; then
  exit 1
fi
