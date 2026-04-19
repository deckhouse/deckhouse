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

{{- /*
Description of problem with XFS https://www.suse.com/support/kb/doc/?id=000020068
*/}}
for FS_NAME in $(mount -l -t xfs | awk '{ print $1 }'); do
  if command -v xfs_info >/dev/null && xfs_info $FS_NAME | grep -q ftype=0; then
     >&2 echo "ERROR: XFS file system with ftype=0 was found ($FS_NAME)."
     exit 1
  fi
done
