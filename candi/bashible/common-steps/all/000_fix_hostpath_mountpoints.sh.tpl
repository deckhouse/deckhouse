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

# /etc/timezone and /etc/localtime should be files (or symbolic links), but specifying a hostPath without a type in the chrony module would create an empty directory. This behaviour was fixed in PR #14920.

if [ -d /etc/localtime ]; then
    rmdir /etc/localtime
fi

if [ -d /etc/timezone ]; then
    rmdir /etc/timezone
fi
