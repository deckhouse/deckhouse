# Copyright 2025 Flant JSC
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

{{- /*
  Uploaded on every path, and marked as already applied when the images came from a bundle.

  This secret does two jobs, and an installation from a bundle wants one of them and not the other.

  It carries the registry PKI — the CA and the read/write credentials — which the installer generates
  afresh for each bashible bundle and cannot recover once handed over: dhctl reads this secret back to
  build the `deckhouse-registry` secret, so the copy the node uploads is the only authoritative one.
  Skipping the upload therefore does not merely omit a legacy artifact, it strands the installation —
  "get PKI: secrets \"registry-init\" not found", with the control plane already up and the store
  already full, which is exactly how a bundle installation failed.

  It is also what starts the previous implementation's state machine: unapplied, it records a target
  mode and waits for that implementation's DaemonSet to place a static pod on every master. In a
  cluster installed from a bundle that DaemonSet is scaled to zero — the operator configured the
  current implementation, which owns the registry instead — so the transition never completes, and the
  current implementation refuses to take over from a predecessor that reads as mid-transition. Which
  of the two records its state first then decides whether the cluster ends up with any registry
  implementation active at all: a race, not a property.

  The annotation is that implementation's own way of saying the secret has been consumed
  (`registry.deckhouse.io/is-applied`, read for presence in
  hooks/orchestrator/init-secret/k8s.go). Setting it up front leaves the PKI available to everyone who
  needs it and the state machine untouched.

  For every other cluster, including the legacy Local and Proxy installations these steps were written
  for, `fromBundle` is absent, no annotation is set, and the step behaves exactly as before.
*/ -}}
{{- /*
  Read before the `with` below, which rebinds the dot to the init section: inside it `.registry` is not
  the node context any more, and the flag silently reads as false — which is the wrong half of this
  decision to get by accident, since it is the one that keeps the legacy state machine shut.
*/ -}}
{{- $fromBundle := ((.registry).bootstrap).fromBundle }}
{{- with ((.registry).bootstrap).init }}

# Create init registry config file
INIT_CONFIG_PATH="$(bb-tmp-file)"
bb-sync-file $INIT_CONFIG_PATH - << "EOF"
{{ . | toYaml }}
EOF

# Force admin-cert auth for operations requiring elevated privileges
export BB_KUBE_AUTH_TYPE="admin-cert"
export BB_KUBE_APISERVER_URL=""
bb-curl-helper-extract-admin-certs

# Create d8-system namespace if it does not exist
bb-curl-kube "/api/v1/namespaces/d8-system" >/dev/null 2>&1 || \
  bb-curl-kube "/api/v1/namespaces" \
    -X POST \
    -H "Content-Type: application/json" \
    --data '{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"d8-system"}}' >/dev/null

# Upload init registry secret
{{- if $fromBundle }}
INIT_ANNOTATIONS='{"registry.deckhouse.io/is-applied":""}'
{{- else }}
INIT_ANNOTATIONS='{}'
{{- end }}

bb-curl-kube "/api/v1/namespaces/d8-system/secrets/registry-init" -X DELETE >/dev/null || true

bb-curl-kube "/api/v1/namespaces/d8-system/secrets" \
  -X POST \
  -H "Content-Type: application/json" \
  --data "$(jq -nc --arg data "$(base64 -w0 < "$INIT_CONFIG_PATH")" \
    --argjson annotations "$INIT_ANNOTATIONS" \
    '{"apiVersion":"v1","kind":"Secret","metadata":{"name":"registry-init","namespace":"d8-system","annotations":$annotations},"type":"Opaque","data":{"config":$data}}')" >/dev/null

{{- end }}
