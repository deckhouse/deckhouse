#!/usr/bin/env bash

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

# Prints evidence about the authorization webhook, to explain an outage of it
# (for example, a Pod evicted by the VPA updater and recreated later by the
# DaemonSet controller). Never fails: it is a diagnostic.
#
# Usage: webhook-evidence.sh [LABEL]
set -Euo pipefail

label="${1:-}"
echo "===== user-authz webhook evidence ${label:+(${label}) }at $(date -u +%Y-%m-%dT%H:%M:%SZ) ====="

echo "--- webhook Pods (name, UID, created, deletion, phase, ready, restarts, node)"
kubectl -n d8-user-authz get pods -l app=user-authz-webhook -o json 2>&1 | jq -r '
  .items[]? | [
    .metadata.name, .metadata.uid, .metadata.creationTimestamp,
    (.metadata.deletionTimestamp // "-"), .status.phase,
    ([.status.conditions[]? | select(.type == "Ready") | .status] | first // "-"),
    ([.status.containerStatuses[]? | "\(.name)=\(.restartCount)"] | join(",")),
    (.spec.nodeName // "-")
  ] | @tsv' 2>/dev/null || true

echo "--- DaemonSet user-authz-webhook (generation, observedGeneration, status)"
kubectl -n d8-user-authz get daemonset user-authz-webhook -o json 2>/dev/null \
  | jq -c '{generation: .metadata.generation, status: .status,
            updateStrategy: .spec.updateStrategy}' 2>/dev/null || echo "DaemonSet not found"

echo "--- events in d8-user-authz (oldest first)"
kubectl -n d8-user-authz get events --sort-by=.lastTimestamp \
  -o custom-columns=LAST:.lastTimestamp,TYPE:.type,REASON:.reason,OBJECT:.involvedObject.kind/.involvedObject.name,SOURCE:.source.component,MESSAGE:.message \
  2>&1 | tail -n 60 || true

echo "--- eviction, kill and VPA related events in d8-user-authz"
kubectl -n d8-user-authz get events -o json 2>/dev/null | jq -r '
  .items[]
  | select((.reason // "") | test("Evict|Killing|Preempt|Delete|SuccessfulCreate|SuccessfulDelete|FailedCreate|Unhealthy"; "i")
           or ((.source.component // "") | test("vpa|updater|admission"; "i"))
           or ((.message // "") | test("vpa|evict|recommend"; "i")))
  | [(.lastTimestamp // .eventTime // "-"), .reason, .involvedObject.name, (.source.component // "-"), .message]
  | @tsv' || true

echo "--- VerticalPodAutoscaler user-authz-webhook"
kubectl -n d8-user-authz get verticalpodautoscalers.autoscaling.k8s.io user-authz-webhook -o yaml 2>&1 \
  | sed -n '1,80p' || true
exit 0
