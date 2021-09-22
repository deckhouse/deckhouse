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

{{- if eq .runType "Normal" }}
# Do nothing, if kubelet is already bootstraped
if [ -f /etc/kubernetes/kubelet.conf ] ; then exit 0 ; fi

# Generate bootstrap kubeconfig for kubelet
cat > /etc/kubernetes/bootstrap-kubelet.conf << EOF
apiVersion: v1
kind: Config
current-context: kubelet-bootstrap@default
clusters:
- cluster:
    certificate-authority-data: $(cat /var/lib/bashible/ca.crt | base64 -w0)
    server: https://127.0.0.1:6445/
  name: default
contexts:
- context:
    cluster: default
    user: kubelet-bootstrap
  name: kubelet-bootstrap@default
users:
- name: kubelet-bootstrap
  user:
    as-user-extra: {}
    token: $(</var/lib/bashible/bootstrap-token)
EOF
chmod 0600 /etc/kubernetes/bootstrap-kubelet.conf
{{- end }}
