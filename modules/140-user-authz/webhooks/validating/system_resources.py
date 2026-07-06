#!/usr/bin/python3

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


# Superadmin: protect system resources placed inside user/project namespaces (RBAC v2).
#
# Deckhouse places system components (Dex authenticator pods, virtualization VM pods/PVCs/kvvm/kvvmi,
# managed-service endpoints, etc.) INTO user/project namespaces. Those objects are marked with the
# label `deckhouse.io/system-resource: "true"` (adding it to each module's resources is a separate
# cross-module effort). A label
# (not an annotation) is used so the system-pods snapshot can select marked pods server-side instead
# of watching every pod. This hook ENFORCES the marking at admission time:
#
#   1. system-resource edit protection (binding rbacv2-system-resource-edit.deckhouse.io):
#      UPDATE/PATCH/DELETE of an object labeled `deckhouse.io/system-resource: "true"` is denied for
#      everyone below superadmin. PATCH arrives as an UPDATE admission operation, so it is covered.
#
#   2. ProjectTemplate-owned (heritage) protection: objects labeled `heritage: multitenancy-manager`
#      are rendered and reconciled by the multitenancy-manager controller. They must not be mutated by
#      anyone but the controller / cluster components — not even by a project superadmin. This mirrors
#      the `projects.deckhouse.io/managed-by=controller` protection in
#      160/.../webhook/rolebinding/validator.go and the /protect webhook in 160/.../webhooks/protect.go.
#
#   3. exec/attach/port-forward protection (binding rbacv2-system-resource-exec.deckhouse.io):
#      `create` on the pods/exec, pods/attach, pods/portforward subresources targeting a
#      system-labeled pod is denied for users below superadmin. The admission object for a CONNECT
#      request is a PodExecOptions/PodPortForwardOptions, not the Pod, so the target pod's marker is
#      resolved through the `system-pods` snapshot (pods selected by the marker label server-side).
#
# Requester level: "superadmin" is determined as membership (directly by username, by ServiceAccount,
# or by one of the request's groups) in a (Cluster)RoleBinding to one of the built-in superadmin
# ClusterRoles (d8:namespace:superadmin / d8:project:superadmin / d8:system:superadmin), scoped to the
# request namespace for RoleBindings; OR membership in a cluster-admin/system group. The relevant
# bindings are kept in the `superadmin-rolebindings` / `superadmin-clusterrolebindings` snapshots.
# Known limitations: snapshot freshness; only the built-in superadmin ClusterRoles are recognised — a
# custom role aggregating the superadmin lineage is not detected; group membership is matched by name
# as presented in request.userInfo.groups.
#
# Out of scope (documented as follow-ups): the GET/LIST "visibility" split (admin+ sees vendor-API
# system resources, everyone sees shared-API ones) is a READ/authorization concern that admission
# webhooks cannot enforce — it is an RBAC-layer / EE-authorizer / permission-browser concern. Adding
# the `deckhouse.io/system-resource` annotation to each module's resources is a cross-module effort.

import json
import subprocess
from typing import Optional

from deckhouse import hook
from dotmap import DotMap

# Marker LABEL (a label, not an annotation): a label lets the system-pods snapshot below use a
# server-side labelSelector — the webhook-handler then watches ONLY marked pods instead of every pod
# in the cluster (it runs on a tiny 50m/100Mi budget). The edit webhook likewise filters on the label
# via matchConditions. Modules mark their in-namespace system objects with this label.
SYSTEM_RESOURCE_LABEL = "deckhouse.io/system-resource"
SYSTEM_RESOURCE_VALUE = "true"

HERITAGE_LABEL = "heritage"
HERITAGE_MULTITENANCY = "multitenancy-manager"

# Built-in ClusterRoles that confer superadmin in a namespace/project/cluster. A binding to any of
# these (scoped to the request namespace for RoleBindings) marks the subject as superadmin.
SUPERADMIN_ROLES = {
    "d8:namespace:superadmin",
    "d8:project:superadmin",
    "d8:system:superadmin",
}

# Identities that author/reconcile system and controller-managed objects; they bypass all checks.
PRIVILEGED_USERS = {
    "system:apiserver",
    "system:serviceaccount:d8-system:deckhouse",
    "system:serviceaccount:kube-system:clusterrole-aggregation-controller",
    "system:serviceaccount:d8-multitenancy-manager:multitenancy-manager",
}

# Cluster-admin and cluster-component groups. Members bypass protection: a true cluster-admin
# (super-admin.conf / system:masters) and system controllers must keep break-glass access and must be
# able to reconcile heritage objects, mirroring 160/.../webhooks/protect.go's systemBypassGroups.
BYPASS_GROUPS = {
    "system:masters",
    "system:nodes",
    "system:serviceaccounts:kube-system",
    "system:serviceaccounts:d8-system",
}

BINDING_EDIT = "rbacv2-system-resource-edit.deckhouse.io"
BINDING_EXEC = "rbacv2-system-resource-exec.deckhouse.io"

# Superadmin status and the exec target pod are resolved with on-demand LIVE reads — no informers and
# no snapshots. The protected events are rare (a non-superadmin editing a system-labeled object, or
# exec into a system pod), so a live read per event is cheap, keeps the webhook-handler free of any
# standing watch (no all-pods / all-(cluster)rolebindings stream), and lets the hook register instantly
# without waiting for informer synchronization (an unsynced informer previously left the whole handler
# with an empty hook registry). Reads are scoped: RoleBindings only in the request namespace, the exec
# target pod by name.
#
# Reads go through the `kubectl` binary that ships in the webhook-handler image (it is what the bash
# webhooks in this same image already use for live reads). This deliberately avoids Python's `ssl`
# module: the image's CPython links `_ssl` against libssl.so.3/libcrypto.so.3, which are NOT present
# in the final image, so a top-level `import ssl` (needed by urllib for HTTPS to the API server)
# raises ImportError at hook-config time. Because the handler is fail-closed, that single import
# error previously crashed config loading and took down EVERY webhook on the handler. kubectl reads
# the in-cluster service-account credentials itself, so no urllib/ssl is needed here. Scoping and RBAC
# requirements are unchanged: the webhook-handler ServiceAccount still needs list on
# (cluster)rolebindings and get on pods.
_KUBECTL = "kubectl"
_API_TIMEOUT_SECONDS = 10

CONFIG = f"""
configVersion: v1
kubernetesValidating:
- name: {BINDING_EDIT}
  group: main
  matchConditions:
  - name: exclude-kube-apiserver
    expression: '"system:apiserver" != request.userInfo.username'
  - name: exclude-deckhouse
    expression: '"system:serviceaccount:d8-system:deckhouse" != request.userInfo.username'
  - name: exclude-aggregation-controller
    expression: '"system:serviceaccount:kube-system:clusterrole-aggregation-controller" != request.userInfo.username'
  - name: exclude-multitenancy-manager
    expression: '"system:serviceaccount:d8-multitenancy-manager:multitenancy-manager" != request.userInfo.username'
  # Only forward requests for objects that actually carry the markings, so the (intentionally broad)
  # rule below does not put every namespaced UPDATE/DELETE through the hook. Guarded with has()/in to
  # never error (which would otherwise fail the request under a Fail matchConditions policy).
  - name: only-marked-objects
    # In admission matchConditions the reviewed objects are TOP-LEVEL CEL variables `object`/`oldObject`
    # (the `request` variable is the AdmissionRequest metadata and has NO `.object`/`.oldObject`). Using
    # `request.object` makes the expression fail CEL compilation, which invalidates the WHOLE
    # ValidatingWebhookConfiguration and crash-loops the webhook-handler (fail-closed → every webhook on
    # the handler stops). `object` is null on DELETE / CONNECT; `oldObject` is null on CREATE — guarded.
    expression: >-
      (object != null && has(object.metadata.labels) && (
        ('{SYSTEM_RESOURCE_LABEL}' in object.metadata.labels && object.metadata.labels['{SYSTEM_RESOURCE_LABEL}'] == '{SYSTEM_RESOURCE_VALUE}')
        || ('{HERITAGE_LABEL}' in object.metadata.labels && object.metadata.labels['{HERITAGE_LABEL}'] == '{HERITAGE_MULTITENANCY}')
      )) || (oldObject != null && has(oldObject.metadata.labels) && (
        ('{SYSTEM_RESOURCE_LABEL}' in oldObject.metadata.labels && oldObject.metadata.labels['{SYSTEM_RESOURCE_LABEL}'] == '{SYSTEM_RESOURCE_VALUE}')
        || ('{HERITAGE_LABEL}' in oldObject.metadata.labels && oldObject.metadata.labels['{HERITAGE_LABEL}'] == '{HERITAGE_MULTITENANCY}')
      ))
  rules:
  - apiGroups:   ["*"]
    apiVersions: ["*"]
    operations:  ["UPDATE", "DELETE"]
    resources:   ["*"]
    scope:       "Namespaced"
- name: {BINDING_EXEC}
  group: main
  matchConditions:
  - name: exclude-kube-apiserver
    expression: '"system:apiserver" != request.userInfo.username'
  - name: exclude-deckhouse
    expression: '"system:serviceaccount:d8-system:deckhouse" != request.userInfo.username'
  rules:
  - apiGroups:   [""]
    apiVersions: ["*"]
    operations:  ["CONNECT"]
    resources:   ["pods/exec", "pods/attach", "pods/portforward"]
    scope:       "Namespaced"
"""


def main(ctx: hook.Context):
    try:
        binding_context = DotMap(ctx.binding_context)
        error_message = validate(binding_context)
        if error_message:
            ctx.output.validations.deny(error_message)
        else:
            ctx.output.validations.allow()
    except Exception as e:
        ctx.output.validations.error(str(e))


def _to_dict(obj) -> dict:
    if obj is None:
        return {}
    if hasattr(obj, "toDict"):
        obj = obj.toDict()
    return obj or {}


def _to_list(value) -> list:
    if not value:
        return []
    if hasattr(value, "toDict"):
        value = value.toDict()
    return list(value) if value else []


def _markings(obj: dict) -> tuple[bool, bool]:
    """Return (is_system_resource, is_heritage_managed) for an object dict."""
    meta = obj.get("metadata") or {}
    labels = meta.get("labels") or {}
    is_system = labels.get(SYSTEM_RESOURCE_LABEL) == SYSTEM_RESOURCE_VALUE
    is_heritage = labels.get(HERITAGE_LABEL) == HERITAGE_MULTITENANCY
    return is_system, is_heritage


def _kubectl_get(resource: str, name: str = "", namespace: str = "") -> Optional[dict]:
    """On-demand live read via the in-cluster `kubectl ... -o json`. Returns the parsed JSON (a List
    object for a collection read, a single object for a named read), or None when a named resource is
    NotFound so the caller can distinguish "gone" from a real failure. Raises on any other failure."""
    cmd = [_KUBECTL, "get", resource]
    if name:
        cmd.append(name)
    if namespace:
        cmd += ["--namespace", namespace]
    cmd += ["--output", "json"]
    proc = subprocess.run(
        cmd, capture_output=True, text=True, timeout=_API_TIMEOUT_SECONDS, check=False
    )
    if proc.returncode != 0:
        stderr = (proc.stderr or "").strip()
        if "NotFound" in stderr or "not found" in stderr.lower():
            return None
        raise RuntimeError(f"kubectl get {' '.join(cmd[2:])} failed: {stderr}")
    return json.loads(proc.stdout)


def _superadmin_bindings(resource: str, namespace: str = "") -> list:
    """Live-list the (Cluster)RoleBindings of `resource`, keeping only those bound to a built-in
    superadmin ClusterRole."""
    items = (_kubectl_get(resource, namespace=namespace) or {}).get("items") or []
    return [b for b in items if (b.get("roleRef") or {}).get("name") in SUPERADMIN_ROLES]


def _subject_matches(subject: dict, username: str, groups: set) -> bool:
    kind = subject.get("kind")
    name = subject.get("name")
    if not name:
        return False
    if kind == "User":
        return name == username
    if kind == "Group":
        return name in groups
    if kind == "ServiceAccount":
        return f"system:serviceaccount:{subject.get('namespace', '')}:{name}" == username
    return False


def _any_subject_matches(bindings: list, username: str, groups: set) -> bool:
    for binding in bindings:
        for subject in binding.get("subjects") or []:
            if _subject_matches(subject, username, groups):
                return True
    return False


def is_superadmin(username: str, groups: set, namespace: str) -> bool:
    # Cluster-wide superadmin: a ClusterRoleBinding to a superadmin role grants it everywhere.
    crbs = _superadmin_bindings("clusterrolebindings")
    if _any_subject_matches(crbs, username, groups):
        return True
    # Namespace-scoped superadmin: a RoleBinding to a superadmin role IN THE REQUEST NAMESPACE only
    # (read just that namespace, never all of them).
    if namespace:
        rbs = _superadmin_bindings("rolebindings", namespace=namespace)
        if _any_subject_matches(rbs, username, groups):
            return True
    return False


def is_system_pod(namespace: str, name: str) -> bool:
    if not namespace or not name:
        return False
    pod = _kubectl_get("pods", name=name, namespace=namespace)
    if pod is None:
        return False  # pod is gone — nothing to protect
    labels = ((pod.get("metadata") or {}).get("labels")) or {}
    return labels.get(SYSTEM_RESOURCE_LABEL) == SYSTEM_RESOURCE_VALUE


def validate(ctx: DotMap) -> Optional[str]:
    request = ctx.review.request
    username = request.userInfo.username
    groups = set(_to_list(request.userInfo.groups))

    # System components and cluster-admins bypass all protections.
    if username in PRIVILEGED_USERS or (groups & BYPASS_GROUPS):
        return None

    binding = ctx.binding
    if binding == BINDING_EXEC:
        return validate_exec(request, username, groups)
    return validate_edit(request, username, groups)


def validate_edit(request: DotMap, username: str, groups: set) -> Optional[str]:
    new_obj = _to_dict(request.object)
    old_obj = _to_dict(request.oldObject)

    # Evaluate markings on both the new and the old object so a user cannot bypass protection by
    # stripping the marking in the same UPDATE that mutates the resource.
    sys_new, her_new = _markings(new_obj)
    sys_old, her_old = _markings(old_obj)
    is_system = sys_new or sys_old
    is_heritage = her_new or her_old

    if not is_system and not is_heritage:
        return None

    obj = new_obj or old_obj
    meta = obj.get("metadata") or {}
    kind = request.kind.kind or obj.get("kind") or "resource"
    name = meta.get("name") or request.name or ""
    namespace = meta.get("namespace") or request.namespace or ""

    # heritage protection wins: ProjectTemplate-owned objects are controller-managed and must not be
    # mutated by users — not even by a project superadmin.
    if is_heritage:
        return (
            f'{kind} "{name}" is managed by the multitenancy-manager controller '
            f'(label {HERITAGE_LABEL}={HERITAGE_MULTITENANCY}) and cannot be modified. '
            "Such resources are reconciled from the ProjectTemplate; change the ProjectTemplate or "
            "Project instead."
        )

    # system-resource protection: editable only by superadmin.
    if is_system and not is_superadmin(username, groups, namespace):
        return (
            f'{kind} "{name}" is a Deckhouse system resource '
            f'(label {SYSTEM_RESOURCE_LABEL}={SYSTEM_RESOURCE_VALUE}) placed in this '
            "namespace. It can be modified or deleted only by a superadmin "
            "(d8:namespace:superadmin / d8:project:superadmin)."
        )

    return None


def validate_exec(request: DotMap, username: str, groups: set) -> Optional[str]:
    namespace = request.namespace or ""
    pod_name = request.name or ""
    subresource = request.subResource or ""

    if not is_system_pod(namespace, pod_name):
        return None

    if is_superadmin(username, groups, namespace):
        return None

    action = subresource or "exec"
    return (
        f'Pod "{pod_name}" is a Deckhouse system resource '
        f'(label {SYSTEM_RESOURCE_LABEL}={SYSTEM_RESOURCE_VALUE}); '
        f'{action} into it is allowed only for a superadmin '
        "(d8:namespace:superadmin / d8:project:superadmin)."
    )


if __name__ == "__main__":
    hook.run(main, config=CONFIG)
