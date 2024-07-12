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

{{- if semverCompare ">1.26" .kubernetesVersion }}
# if config was changed we should restart kubelet
bb-event-on 'bb-sync-file-changed' 'bb-flag-set kubelet-need-restart'

# flag should set always for aws
bb-flag-set kubelet-enable-credential-provider

# matchImages should contain globs, not regexp, but in k8s code we found next patter
# ^(\d{12})\.dkr\.ecr(\-fips)?\.([a-zA-Z0-9][a-zA-Z0-9-_]*)\.(amazonaws\.com(\.cn)?|sc2s\.sgov\.gov|c2s\.ic\.gov)$
bb-sync-file /var/lib/bashible/kubelet-credential-provider-config.yaml - << "EOF"
apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: ecr-credential-provider
    matchImages:
      - "*.dkr.ecr.*.amazonaws.com"
      - "*.dkr.ecr.*.amazonaws.com.cn"
      - "*.dkr.ecr.*.sc2s.sgov.gov"
      - "*.dkr.ecr.*.c2s.ic.gov"
      - "*.dkr.ecr-fips.*.amazonaws.com"
      - "*.dkr.ecr-fips.*.amazonaws.com.cn"
      - "*.dkr.ecr-fips.*.sc2s.sgov.gov"
      - "*.dkr.ecr-fips.*.c2s.ic.gov"
    defaultCacheDuration: "0"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
EOF
{{- end }}
