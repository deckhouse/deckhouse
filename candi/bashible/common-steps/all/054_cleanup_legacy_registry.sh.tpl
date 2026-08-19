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
# Its registry ran as static pods configured out of `/etc/kubernetes/registry` — `auth` and
# `distribution`, holding their configuration and PKI. The pods themselves go away with the manifests
# that implementation stops rendering, but these directories were written on the node and have nobody
# to remove them, so they stay. Measured on a migrated cluster: both still there after the handover,
# with nothing reading them.
#
# Paths spelled out rather than globbed or looped over. This runs as root on every node, and the one
# failure mode worth engineering against is a wildcard that matches something else — `registry-agent`
# and `registry-proxy` are siblings of these two and must be left alone.
#
# Gated on the same condition as the agent's own step, which is the honest gate: while the agent is
# configured here, this implementation owns the node's registry, so that implementation's files are
# dead. Not gated on its absence — there is no way back to it, so a file kept "in case" is kept
# forever.
#
# Deliberately NOT removed: `/opt/deckhouse/images/registry-proxy.tar` and the package it comes from.
# The previous implementation's modes remain the model of a bootstrap inside dhctl — `Local` is how an
# installation from a bundle is expressed — so that image is needed before any of this runs, and on a
# running node it is residue of a path that has to keep working.

{{- if .registry.agent }}

if [ -d /etc/kubernetes/registry/auth ]; then
  rm -rf /etc/kubernetes/registry/auth
  bb-log-info "removed /etc/kubernetes/registry/auth, left by the previous registry implementation"
fi

if [ -d /etc/kubernetes/registry/distribution ]; then
  rm -rf /etc/kubernetes/registry/distribution
  bb-log-info "removed /etc/kubernetes/registry/distribution, left by the previous registry implementation"
fi

# The parent only if it is empty: on a node that never ran that implementation it does not exist, and
# on one that did it may hold something a later version put there.
rmdir /etc/kubernetes/registry 2>/dev/null || true

{{- end }}
