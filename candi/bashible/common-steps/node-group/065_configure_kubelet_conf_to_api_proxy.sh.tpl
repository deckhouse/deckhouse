# Copyright 2021 Flant JSC
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

{{- if ne .runType "ClusterBootstrap" }}

# Do nothing, if kubelet wasn't bootstraped yet
if [ ! -f /etc/kubernetes/kubelet.conf ] ; then exit 0 ; fi
if [ ! -f /var/lib/kubelet/pki/kubelet-client-current.pem ] ; then exit 0 ; fi

bb-event-on 'bb-sync-file-changed' 'bb-flag-set kubelet-need-restart'

bb-sync-file /etc/kubernetes/kubelet.conf - << EOF
apiVersion: v1
kind: Config

clusters:
- cluster:
    certificate-authority-data: $(cat /etc/kubernetes/pki/ca.crt | base64 -w0)
    server: https://127.0.0.1:6445
  name: d8-cluster

users:
- name: d8-user
  user:
    client-certificate: /var/lib/kubelet/pki/kubelet-client-current.pem
    client-key: /var/lib/kubelet/pki/kubelet-client-current.pem

contexts:
- context:
    cluster: d8-cluster
    namespace: default
    user: d8-user
  name: d8-context

current-context: d8-context
preferences: {}
EOF

# CIS becnhmark purposes
chmod 600 /etc/kubernetes/kubelet.conf
{{- end }}
