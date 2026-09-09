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

# The node agent's own directory is left alone, and it is the one thing under /etc/kubernetes that has
# a reader outside root.
#
# Everything else here is control-plane key material, where 700/600 is exactly right. The agent's is
# different: its certificate authority is what registry-packages-proxy verifies the agent with, and that
# proxy runs unprivileged in a pod mounting this directory from the host. Swept up by the rule above, the
# authority became unreadable to it — and an unreadable authority reads to it as "there is no agent
# here", so it fetched from somewhere else without a word about it.
#
# That is not hypothetical: step 053 sets these permissions itself, and this step used to undo them
# eight seconds later, on every bashible pass. Ownership of the directory belongs to the step that
# creates it — the private key there stays 0600, set where it is written.
find /etc/kubernetes -path /etc/kubernetes/registry-agent -prune -o -type d -exec chmod 700 {} \;
find /etc/kubernetes -path /etc/kubernetes/registry-agent -prune -o -type f -exec chmod 600 {} \;

chmod 700 /var/lib/kubelet/

if [[ -d /etc/containerd ]]; then
    chmod 700 /etc/containerd
fi

if [[ -d /var/lib/etcd ]]; then
    chmod 700 /var/lib/etcd
fi
