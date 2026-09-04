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
# Its registry ran as one static pod configured out of `/etc/kubernetes/registry`, mounting four
# directories from it: `pki`, `auth`, `distribution` and `mirrorer`. The pod itself goes away with the
# manifest that implementation stops rendering, but these directories were written on the node and
# have nobody to remove them, so they stay. Measured on a migrated cluster: still there after the
# handover, with nothing reading them.
#
# `pki` is the one that matters, and it used to be left here: the bootstrap step copies the
# authority's certificate and the auth, distribution and token PRIVATE KEYS into it, and they stayed
# on the node for as long as the node lived — long after the implementation that used them was gone.
# The `rmdir` at the bottom could never succeed either, which was the visible half of the same
# omission.
#
# Paths spelled out rather than globbed or looped over. This runs as root on every node, and the one
# failure mode worth engineering against is a wildcard that matches something else — `registry-agent`
# and `registry-proxy` are siblings of these four and must be left alone.
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
