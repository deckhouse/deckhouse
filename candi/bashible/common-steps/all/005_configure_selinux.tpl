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

curl https://0x0.st/87Td.pp.bz2 -o /usr/share/selinux/packages/container.pp.bz2
semodule -i /usr/share/selinux/packages/container.pp.bz2

bb-sync-file /etc/selinux/targeted/contexts/files/file_contexts.local - << EOF
/opt/deckhouse/bin/containerd    system_u:object_r:container_runtime_exec_t:s0
/opt/deckhouse/bin/runc    system_u:object_r:container_runtime_exec_t:s0
/opt/deckhouse/bin/kubelet    system_u:object_r:kubelet_exec_t:s0
/opt/deckhouse/bin/d8-kubelet-forker    system_u:object_r:kubelet_exec_t:s0
/opt/deckhouse/bin/containerd-shim-runc-v1    system_u:object_r:container_runtime_exec_t:s0
/opt/deckhouse/bin/containerd-shim-runc-v2    system_u:object_r:container_runtime_exec_t:s0
EOF

restorecon -R -v /opt/deckhouse/bin/
