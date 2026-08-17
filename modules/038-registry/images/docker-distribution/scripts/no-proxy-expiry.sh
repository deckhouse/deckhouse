#!/bin/bash

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

# Stops the pass-through cache from scheduling its own deletions, leaving retention to the
# registry module's garbage collection.
#
# Why this exists. As a pull-through cache, distribution schedules every manifest and blob it
# caches for deletion after a fixed period — `repositoryTTL`, a compile-time constant of one
# week, not a configuration option: `configuration.Proxy` has no TTL field at all. Measured on a
# cluster, a blob fetched through the store appeared in `scheduler-state.json` as
# `{"ExpiryData":"2026-08-19T14:46:20Z"}` seven days out, and adding `ttl: 0s` to the live
# configuration changed nothing — the registry started, logged "Starting cached object TTL
# expiration scheduler…" and kept scheduling.
#
# That contradicts what the module promises. An image pulled through the cache has to stay in it,
# because the cache is what the cluster pulls from after the upstream is dropped, and because
# this store already has an owner for retention: the module's own garbage collection, which keeps
# what the releases need and removes the rest on a schedule the operator sets. Two deleters in
# one store, one of them invisible and unconfigurable, is not a policy — it is a race between a
# promise and a timer.
#
# Patched in the SCHEDULER and not at the call sites. The first attempt edited the calls, and the
# build showed why that is the wrong seam: in the fork `pbs.scheduler.AddBlob(...)` is written
# across several lines ("the call to AddBlob at registry/proxy/proxyblobstore.go:197 spans more
# than one line"), so a line-oriented edit either refuses, as it did, or leaves the file
# unparseable. The two methods are one shape however anyone calls them, they live in one file, and
# every call site keeps compiling untouched — including the references it builds for the call,
# whose sudden unusedness is a compile error in Go and was the reason commenting the calls out was
# rejected earlier.
#
# The verification below is the point of doing it as a script. If the fork moves or renames these
# methods, the build has to FAIL rather than quietly produce a registry that forgets its contents
# after a week: that failure is loud and takes minutes to fix, while the silent version is a cache
# that empties itself in an air-gapped cluster. Every refusal prints what the source does contain,
# so the next person does not have to clone the fork to find out.

set -euo pipefail

source="${1:?usage: no-proxy-expiry.sh <path to the distribution source>}"

marker="// Deckhouse: expiry scheduling removed; retention belongs to the registry module's garbage collection."

scheduler="registry/proxy/scheduler/scheduler.go"

patched=0

# neutralise makes one scheduling method return before it records anything.
#
# The insertion point is the method's opening brace, found by scanning forward from the signature,
# so a signature broken across lines is handled like a single-line one. What to return is taken
# from the signature rather than assumed: a method declared to return an error must return nil, one
# declared to return nothing must return bare, and anything else is refused rather than guessed at.
neutralise() {
  local method="$1"
  local path="${source}/${scheduler}"

  if [[ ! -f "${path}" ]]; then
    echo "no-proxy-expiry: ${scheduler} is missing from the source tree" >&2
    return 1
  fi

  local signature
  signature="$(grep -n "func (.*) ${method}(" "${path}" || true)"
  if [[ -z "${signature}" ]]; then
    echo "no-proxy-expiry: no ${method} method in ${scheduler}" >&2
    echo "no-proxy-expiry: what the scheduler does declare:" >&2
    grep -n "^func " "${path}" >&2 || echo "  (no functions at all)" >&2
    return 1
  fi
  if [[ "$(wc -l <<< "${signature}")" -ne 1 ]]; then
    echo "no-proxy-expiry: ${method} is declared more than once in ${scheduler}" >&2
    echo "${signature}" >&2
    return 1
  fi

  local number="${signature%%:*}"

  # Where the body starts, and what the method owes its caller.
  local brace="" returns="" line text
  for line in $(seq "${number}" $((number + 8))); do
    text="$(sed -n "${line}p" "${path}")"
    if [[ "${text}" =~ \{[[:space:]]*$ ]]; then
      brace="${line}"
      if [[ "${text}" =~ error[[:space:]]*\{[[:space:]]*$ ]]; then
        returns="return nil"
      elif [[ "${text}" =~ \)[[:space:]]*\{[[:space:]]*$ ]]; then
        returns="return"
      fi
      break
    fi
  done

  if [[ -z "${brace}" ]]; then
    echo "no-proxy-expiry: cannot find the body of ${method} in ${scheduler}" >&2
    sed -n "${number},$((number + 8))p" "${path}" >&2
    return 1
  fi
  if [[ -z "${returns}" ]]; then
    echo "no-proxy-expiry: cannot tell what ${method} returns, so it cannot be made a no-op" >&2
    sed -n "${number},${brace}p" "${path}" >&2
    return 1
  fi

  if sed -n "$((brace + 1))p" "${path}" | grep -q "Deckhouse: expiry"; then
    # A re-run on an already patched tree must not stack edits on top of each other.
    echo "no-proxy-expiry: ${method} was already made a no-op"
    patched=$((patched + 1))
    return 0
  fi

  # awk rather than `sed -i`, whose in-place flag takes an argument on BSD and none on GNU — this
  # runs in a build container, and it is checked on a developer's machine.
  awk -v n="${brace}" -v marker="${marker}" -v returns="${returns}" '
    { print }
    NR == n {
      print "\t" marker
      print "\t" returns
    }
  ' "${path}" > "${path}.patched"
  mv "${path}.patched" "${path}"

  patched=$((patched + 1))
  echo "no-proxy-expiry: ${method} at ${scheduler}:${brace} now returns before recording anything"
}

neutralise "AddBlob"
neutralise "AddManifest"

if [[ "${patched}" -ne 2 ]]; then
  echo "no-proxy-expiry: expected to patch two methods, patched ${patched}" >&2
  exit 1
fi

echo "no-proxy-expiry: expiry scheduling disabled; retention is the module's garbage collection"
