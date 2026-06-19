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

import unittest
from unittest import mock

import system_resources
from deckhouse import hook, tests
from dotmap import DotMap

NS = "team-a"
USER = "alice@example.com"
SYSTEM_LABEL = {system_resources.SYSTEM_RESOURCE_LABEL: system_resources.SYSTEM_RESOURCE_VALUE}
HERITAGE_LABELS = {system_resources.HERITAGE_LABEL: system_resources.HERITAGE_MULTITENANCY}


# ---- API object builders (shape returned by the live kube API, mocked below) -------------------
def user_subject(name):
    return {"kind": "User", "name": name}


def group_subject(name):
    return {"kind": "Group", "name": name}


def rolebinding(role, subjects, namespace=NS):
    return {"metadata": {"namespace": namespace}, "roleRef": {"name": role}, "subjects": subjects}


def clusterrolebinding(role, subjects):
    return {"metadata": {}, "roleRef": {"name": role}, "subjects": subjects}


def superadmin_rb(namespace=NS, user=USER, role="d8:namespace:superadmin"):
    return [rolebinding(role, [user_subject(user)], namespace)]


class FakeAPI:
    """Routes system_resources._kubectl_get(resource, name, namespace) to canned responses so the
    hook's live reads run against fixtures instead of a real cluster.

      rolebindings_by_ns: {namespace: [rolebinding, ...]}
      clusterrolebindings: [clusterrolebinding, ...]
      pods: {(namespace, name): labels_dict}  — a missing key yields None (pod NotFound)
    """

    def __init__(self, *, rolebindings_by_ns=None, clusterrolebindings=None, pods=None):
        self.rolebindings_by_ns = rolebindings_by_ns or {}
        self.clusterrolebindings = clusterrolebindings or []
        self.pods = pods or {}

    def get(self, resource, name="", namespace=""):
        if resource == "clusterrolebindings":
            return {"items": self.clusterrolebindings}
        if resource == "rolebindings":
            return {"items": self.rolebindings_by_ns.get(namespace, [])}
        if resource == "pods":
            key = (namespace, name)
            if key not in self.pods:
                return None  # NotFound
            return {"metadata": {"labels": self.pods[key]}}
        raise AssertionError(f"unexpected kubectl resource: {resource}")


def edit_context(kind, name, *, labels=None, namespace=NS, username=USER, groups=None,
                 operation="UPDATE", old_labels="same"):
    def obj(labs):
        return {
            "kind": kind,
            "metadata": {"name": name, "namespace": namespace, "labels": labs or {}},
        }

    request = {
        "uid": "00000000-0000-0000-0000-000000000001",
        "operation": operation,
        "kind": {"kind": kind},
        "name": name,
        "namespace": namespace,
        "userInfo": {"username": username, "groups": groups or []},
    }
    old = obj(labels if old_labels == "same" else old_labels)
    if operation == "DELETE":
        request["oldObject"] = old
    else:
        request["object"] = obj(labels)
        request["oldObject"] = old

    return DotMap({"binding": system_resources.BINDING_EDIT, "review": {"request": request}})


def exec_context(pod_name, *, namespace=NS, username=USER, groups=None, subresource="exec"):
    return DotMap({
        "binding": system_resources.BINDING_EXEC,
        "review": {
            "request": {
                "uid": "00000000-0000-0000-0000-000000000002",
                "operation": "CONNECT",
                "kind": {"kind": "PodExecOptions"},
                "resource": {"resource": "pods"},
                "subResource": subresource,
                "name": pod_name,
                "namespace": namespace,
                "userInfo": {"username": username, "groups": groups or []},
            }
        },
    })


def _run(ctx, api=None):
    with mock.patch.object(system_resources, "_kubectl_get", (api or FakeAPI()).get):
        return hook.testrun(system_resources.main, [ctx])


# ---- expected-message builders (mirror system_resources.validate_*) ----------------------------
def expected_heritage_deny(kind, name):
    return (
        f'{kind} "{name}" is managed by the multitenancy-manager controller '
        f'(label {system_resources.HERITAGE_LABEL}={system_resources.HERITAGE_MULTITENANCY}) and cannot be modified. '
        "Such resources are reconciled from the ProjectTemplate; change the ProjectTemplate or "
        "Project instead."
    )


def expected_system_deny(kind, name):
    return (
        f'{kind} "{name}" is a Deckhouse system resource '
        f'(label {system_resources.SYSTEM_RESOURCE_LABEL}={system_resources.SYSTEM_RESOURCE_VALUE}) placed in this '
        "namespace. It can be modified or deleted only by a superadmin "
        "(d8:namespace:superadmin / d8:project:superadmin)."
    )


def expected_exec_deny(name, action):
    return (
        f'Pod "{name}" is a Deckhouse system resource '
        f'(label {system_resources.SYSTEM_RESOURCE_LABEL}={system_resources.SYSTEM_RESOURCE_VALUE}); '
        f'{action} into it is allowed only for a superadmin '
        "(d8:namespace:superadmin / d8:project:superadmin)."
    )


class TestSystemResourceEdit(unittest.TestCase):
    def test_system_resource_update_by_non_superadmin_is_denied(self):
        out = _run(edit_context("Pod", "dex-authenticator-0", labels=SYSTEM_LABEL))
        tests.assert_validation_deny(self, out, expected_system_deny("Pod", "dex-authenticator-0"))

    def test_system_resource_delete_by_non_superadmin_is_denied(self):
        out = _run(edit_context("PersistentVolumeClaim", "vm-disk", labels=SYSTEM_LABEL, operation="DELETE"))
        tests.assert_validation_deny(self, out, expected_system_deny("PersistentVolumeClaim", "vm-disk"))

    def test_system_resource_update_by_namespace_superadmin_is_allowed(self):
        out = _run(edit_context("Pod", "dex-authenticator-0", labels=SYSTEM_LABEL),
                   FakeAPI(rolebindings_by_ns={NS: superadmin_rb()}))
        tests.assert_validation_allowed(self, out, None)

    def test_system_resource_update_by_project_superadmin_is_allowed(self):
        api = FakeAPI(rolebindings_by_ns={NS: [rolebinding("d8:project:superadmin", [user_subject(USER)])]})
        out = _run(edit_context("Service", "managed-db", labels=SYSTEM_LABEL), api)
        tests.assert_validation_allowed(self, out, None)

    def test_superadmin_via_clusterrolebinding_is_allowed(self):
        api = FakeAPI(clusterrolebindings=[clusterrolebinding("d8:project:superadmin", [user_subject(USER)])])
        out = _run(edit_context("Pod", "vm-pod", labels=SYSTEM_LABEL), api)
        tests.assert_validation_allowed(self, out, None)

    def test_superadmin_via_group_is_allowed(self):
        api = FakeAPI(rolebindings_by_ns={NS: [rolebinding("d8:namespace:superadmin", [group_subject("platform-admins")])]})
        out = _run(edit_context("Pod", "vm-pod", labels=SYSTEM_LABEL, groups=["platform-admins"]), api)
        tests.assert_validation_allowed(self, out, None)

    def test_superadmin_binding_in_other_namespace_does_not_apply(self):
        # The only superadmin RoleBinding lives in "other-ns"; the request is in NS, which is the only
        # namespace read -> not superadmin here -> deny.
        api = FakeAPI(rolebindings_by_ns={"other-ns": superadmin_rb(namespace="other-ns")})
        out = _run(edit_context("Pod", "dex-authenticator-0", labels=SYSTEM_LABEL), api)
        tests.assert_validation_deny(self, out, expected_system_deny("Pod", "dex-authenticator-0"))

    def test_label_strip_in_same_update_is_still_denied(self):
        # User removes the marker label in the same UPDATE; old object still carries it -> deny.
        out = _run(edit_context("Pod", "dex-authenticator-0", labels={}, old_labels=SYSTEM_LABEL))
        tests.assert_validation_deny(self, out, expected_system_deny("Pod", "dex-authenticator-0"))

    def test_heritage_resource_mutation_by_non_superadmin_is_denied(self):
        out = _run(edit_context("ConfigMap", "tmpl-cm", labels=HERITAGE_LABELS))
        tests.assert_validation_deny(self, out, expected_heritage_deny("ConfigMap", "tmpl-cm"))

    def test_heritage_resource_mutation_even_by_superadmin_is_denied(self):
        out = _run(edit_context("ConfigMap", "tmpl-cm", labels=HERITAGE_LABELS),
                   FakeAPI(rolebindings_by_ns={NS: superadmin_rb()}))
        tests.assert_validation_deny(self, out, expected_heritage_deny("ConfigMap", "tmpl-cm"))

    def test_heritage_resource_mutation_by_cluster_admin_is_allowed(self):
        out = _run(edit_context("ConfigMap", "tmpl-cm", labels=HERITAGE_LABELS, groups=["system:masters"]))
        tests.assert_validation_allowed(self, out, None)

    def test_unmarked_resource_is_allowed(self):
        out = _run(edit_context("ConfigMap", "app-config"))
        tests.assert_validation_allowed(self, out, None)

    def test_deckhouse_sa_bypasses(self):
        out = _run(edit_context("Pod", "dex-authenticator-0", labels=SYSTEM_LABEL,
                                username="system:serviceaccount:d8-system:deckhouse"))
        tests.assert_validation_allowed(self, out, None)

    def test_controller_sa_bypasses_heritage(self):
        out = _run(edit_context("ConfigMap", "tmpl-cm", labels=HERITAGE_LABELS,
                                username="system:serviceaccount:d8-multitenancy-manager:multitenancy-manager"))
        tests.assert_validation_allowed(self, out, None)


class TestSystemResourceExec(unittest.TestCase):
    def test_exec_into_system_pod_by_non_superadmin_is_denied(self):
        api = FakeAPI(pods={(NS, "dex-authenticator-0"): SYSTEM_LABEL})
        out = _run(exec_context("dex-authenticator-0"), api)
        tests.assert_validation_deny(self, out, expected_exec_deny("dex-authenticator-0", "exec"))

    def test_attach_into_system_pod_by_non_superadmin_is_denied(self):
        api = FakeAPI(pods={(NS, "vm-pod"): SYSTEM_LABEL})
        out = _run(exec_context("vm-pod", subresource="attach"), api)
        tests.assert_validation_deny(self, out, expected_exec_deny("vm-pod", "attach"))

    def test_portforward_into_system_pod_by_non_superadmin_is_denied(self):
        api = FakeAPI(pods={(NS, "vm-pod"): SYSTEM_LABEL})
        out = _run(exec_context("vm-pod", subresource="portforward"), api)
        tests.assert_validation_deny(self, out, expected_exec_deny("vm-pod", "portforward"))

    def test_exec_into_system_pod_by_superadmin_is_allowed(self):
        api = FakeAPI(pods={(NS, "dex-authenticator-0"): SYSTEM_LABEL},
                      rolebindings_by_ns={NS: superadmin_rb()})
        out = _run(exec_context("dex-authenticator-0"), api)
        tests.assert_validation_allowed(self, out, None)

    def test_exec_into_non_system_pod_is_allowed(self):
        # Pod exists but carries no marker label -> not a system pod -> allow.
        api = FakeAPI(pods={(NS, "my-app-0"): {}})
        out = _run(exec_context("my-app-0"), api)
        tests.assert_validation_allowed(self, out, None)

    def test_exec_into_missing_pod_is_allowed(self):
        # Pod not found (404) -> nothing to protect -> allow.
        out = _run(exec_context("ghost-pod"), FakeAPI())
        tests.assert_validation_allowed(self, out, None)

    def test_exec_into_system_pod_in_other_namespace_is_allowed(self):
        # The system pod lives in NS; this request targets the same name in team-b -> 404 -> allow.
        api = FakeAPI(pods={(NS, "dex-authenticator-0"): SYSTEM_LABEL})
        out = _run(exec_context("dex-authenticator-0", namespace="team-b"), api)
        tests.assert_validation_allowed(self, out, None)

    def test_exec_by_cluster_admin_is_allowed(self):
        api = FakeAPI(pods={(NS, "dex-authenticator-0"): SYSTEM_LABEL})
        out = _run(exec_context("dex-authenticator-0", groups=["system:masters"]), api)
        tests.assert_validation_allowed(self, out, None)


if __name__ == "__main__":
    unittest.main()
