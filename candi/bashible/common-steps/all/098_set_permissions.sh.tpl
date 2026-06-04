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

find /etc/kubernetes -type d -exec chmod 700 {} \;
find /etc/kubernetes -type f -exec chmod 600 {} \;

chmod 700 /var/lib/kubelet/

if [[ -d /etc/containerd ]]; then
    chmod 700 /etc/containerd
fi

if [[ -d /var/lib/etcd ]]; then
    chown -R etcd:etcd /var/lib/etcd
    chmod 700 /var/lib/etcd
fi

if [[ -d /etc/kubernetes/pki/etcd ]]; then
    chmod 711 /etc/kubernetes/pki
    chown root:etcd /etc/kubernetes/pki/etcd
    chmod 750 /etc/kubernetes/pki/etcd
    chown root:etcd /etc/kubernetes/pki/etcd/*.key 2>/dev/null || true
    chmod 640 /etc/kubernetes/pki/etcd/*.key 2>/dev/null || true
    chown root:etcd /etc/kubernetes/pki/etcd/*.crt 2>/dev/null || true
    chmod 644 /etc/kubernetes/pki/etcd/*.crt 2>/dev/null || true
fi
