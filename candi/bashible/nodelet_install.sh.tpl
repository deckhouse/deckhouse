{{- /*
# Copyright 2026 Flant JSC
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
*/}}
{{- $nodeletDigest := required "the release has no nodelet package (images.registrypackages.nodelet)" .images.registrypackages.nodelet }}
nodelet-timed() {
  local section="$1" start
  shift
  start=$(date +%s.%N)
  "$@"
  echo "[bashible-timing] step=nodelet_install section=${section} dur=$(awk -v s="$start" -v e="$(date +%s.%N)" 'BEGIN{printf "%.3f", e-s}')s"
}
nodelet-document() {
  mkdir -p /var/lib/nodelet
{{- if eq .runType "ClusterBootstrap" }}
  ( umask 077; jq -n --arg name "$(bb-d8-node-name)" --arg seed '{{ required "the first master's node agent needs nodeletSeed in the render context" .nodeletSeed | toJson | b64enc }}' \
    '$seed | @base64d | fromjson | .metadata.name = $name | .spec.nodeName = $name' > /var/lib/nodelet/nodeconfig.yaml.new )
{{- else }}
  local nodelet_attempts="${MAX_RETRIES:-0}"
  if [ "${FIRST_BASHIBLE_RUN-}" = "yes" ]; then
    nodelet_attempts=0
  fi
  ( umask 077; /opt/deckhouse/bin/nodelet first-document -node-group "{{ .nodeGroup.name }}" \
    -api-servers "{{ .clusterMasterKubeAPIEndpoints | join "," }}" -attempts "$nodelet_attempts" > /var/lib/nodelet/nodeconfig.yaml.new )
{{- end }}
  mv -f /var/lib/nodelet/nodeconfig.yaml.new /var/lib/nodelet/nodeconfig.yaml
}
# The agent of this release, not just any agent: one an older release installed may not read the
# NodeConfig this release writes, and then cannot update itself through it. rpp-get records the
# digest it installed; the running agent keeps the replaced binary until it restarts.
nodelet_installed_digest="$(cat "${BB_RP_INSTALLED_PACKAGES_STORE:-/var/cache/registrypackages}/nodelet/digest" 2>/dev/null || true)"
if [ ! -x /opt/deckhouse/bin/nodelet ] || [ ! -f /etc/systemd/system/nodelet.service ] || [ "$nodelet_installed_digest" != "{{ $nodeletDigest }}" ]; then
  nodelet-timed package rpp-get install -force "nodelet:{{ $nodeletDigest }}"
  systemctl try-restart nodelet.service
fi
if [ ! -s /var/lib/nodelet/nodeconfig.yaml ]; then
  nodelet-timed document nodelet-document
fi
nodelet-timed start systemctl enable --now nodelet.service
