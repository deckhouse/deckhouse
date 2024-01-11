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

# CIS becnhmark purposes
find /etc/cni/net.d -type f -print0 2> /dev/null | xargs -0 --no-run-if-empty chmod 600
find /var/lib/cni/networks -type f -print0 2> /dev/null | xargs -0 --no-run-if-empty chmod 600
