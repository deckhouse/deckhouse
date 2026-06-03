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

# Pre-create /var/lib/etcd owned by etcd:etcd before the etcd static pod starts.
# etcd runs with capabilities: drop: ALL (no CAP_DAC_OVERRIDE), so even UID 0
# cannot access a directory it does not own. The directory must belong to the
# etcd user before the first kubelet-managed container start.
# kubelet's DirectoryOrCreate skips creation when the path already exists,
# so pre-creating here guarantees correct ownership from the very first moment.
mkdir -p /var/lib/etcd
chown etcd:etcd /var/lib/etcd
chmod 700 /var/lib/etcd
