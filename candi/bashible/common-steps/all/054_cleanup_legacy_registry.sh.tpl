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

# What the previous implementation leaves on a node, removed once the agent owns it.
#
# Its registry ran as a static pod configured out of `/etc/kubernetes/registry`, mounting `pki`,
# `auth`, `distribution` and `mirrorer`. The pod goes with the manifest that implementation stops
# rendering, but those directories were written on the node and have nobody to remove them — and
# `pki` holds the authority's certificate and the auth, distribution and token PRIVATE KEYS, which
# would stay for as long as the node lives.
#
# Paths spelled out rather than globbed, because this runs as root on every node and the failure
# worth engineering against is a wildcard matching something else: `registry-agent` and
# `registry-proxy` are siblings of these four.
#
# Gated on the same condition as the agent's own step — while the agent is configured here, the other
# implementation's files are dead — and not on its absence, because there is no way back to it.
#
# Deliberately NOT removed: `/opt/deckhouse/images/registry-proxy.tar` and its package. The previous
# implementation's modes are still the model of a bootstrap inside dhctl, so that image is needed
# before any of this runs.

{{- if .registry.agent }}

# The private keys first; everything below them is configuration.
if [ -d /etc/kubernetes/registry/pki ]; then
  rm -rf /etc/kubernetes/registry/pki
  bb-log-info "removed /etc/kubernetes/registry/pki, left by the previous registry implementation"
fi

if [ -d /etc/kubernetes/registry/auth ]; then
  rm -rf /etc/kubernetes/registry/auth
  bb-log-info "removed /etc/kubernetes/registry/auth, left by the previous registry implementation"
fi

if [ -d /etc/kubernetes/registry/distribution ]; then
  rm -rf /etc/kubernetes/registry/distribution
  bb-log-info "removed /etc/kubernetes/registry/distribution, left by the previous registry implementation"
fi

if [ -d /etc/kubernetes/registry/mirrorer ]; then
  rm -rf /etc/kubernetes/registry/mirrorer
  bb-log-info "removed /etc/kubernetes/registry/mirrorer, left by the previous registry implementation"
fi

# The parent only if it is empty: on a node that never ran that implementation it does not exist, and
# on one that did it may hold something a later version put there.
rmdir /etc/kubernetes/registry 2>/dev/null || true

{{- end }}
