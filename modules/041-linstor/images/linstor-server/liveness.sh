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

# This is default linstor controller liveness probe
if ! curl --connect-timeout 3 -sf http://localhost:9999/ > /dev/null; then
  exit 1;
fi

# Sometimes nodes can be shown as Online without established connection to them.
# This is a workaround for https://github.com/LINBIT/linstor-server/issues/331, https://github.com/LINBIT/linstor-server/issues/219
if test -f "/var/log/linstor-controller/linstor-Controller.log"; then
  tail -n 1000 /var/log/linstor-controller/linstor-Controller.log | grep -q 'Target decrypted buffer is too small' && exit 1
  # Because shell keeps last exit code, we must force exit with code 0. If not, we will have exit code 1 because of grep, that not founded anything
  exit 0
else
  exit 0
fi
