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

import system_resources
from deckhouse import hook, tests
from dotmap import DotMap

NS = "team-a"
USER = "alice@example.com"
SYSTEM_ANNOTATION = {system_resources.SYSTEM_RESOURCE_ANNOTATION: system_resources.SYSTEM_RESOURCE_VALUE}
HERITAGE_LABELS = {system_resources.HERITAGE_LABEL: system_resources.HERITAGE_MULTITENANCY}


def rolebinding_snapshot(namespace, role, subjects):
    return {"filterResult": {"namespace": namespace, "roleRef": role, "subjects": subjects}}


def clusterrolebinding_snapshot(role, subjects):
    return {"filterResult": {"roleRef": role, "subjects": subjects}}


def pod_snapshot(namespace, name):
    return {"filterResult": {"namespace": namespace, "name": name}}


def user_subject(name):
    return {"kind": "User", "name": name}


def group_subject(name):
    return {"kind": "Group", "name": name}


def superadmin_rolebinding(namespace=NS, user=USER, role="d8:namespace:superadmin"):
    return [rolebinding_snapshot(namespace, role, [user_subject(user)])]


# The expected-message builders mirror system_resources.validate_* so the assertions stay in sync
# with the source.
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
        f'(annotation {system_resources.SYSTEM_RESOURCE_ANNOTATION}={system_resources.SYSTEM_RESOURCE_VALUE}) placed in this '
        "namespace. It can be modified or deleted only by a superadmin "
        "(d8:namespace:superadmin / d8:project:superadmin)."
    )


def expected_exec_deny(name, action):
    return (
        f'Pod "{name}" is a Deckhouse system resource '
        f'(annotation {system_resources.SYSTEM_RESOURCE_ANNOTATION}={system_resources.SYSTEM_RESOURCE_VALUE}); '
        f'{action} into it is allowed only for a superadmin '
        "(d8:namespace:superadmin / d8:project:superadmin)."
    )


def edit_context(kind, name, *, annotations=None, labels=None, namespace=NS, username=USER,
                 groups=None, operation="UPDATE", old_annotations="same", old_labels="same",
                 rolebindings=None, clusterrolebindings=None):
    def obj(anns, labs):
        return {
            "kind": kind,
            "metadata": {
                "name": name,
                "namespace": namespace,
                "annotations": anns or {},
                "labels": labs or {},
            },
        }

    request = {
        "uid": "00000000-0000-0000-0000-000000000001",
        "operation": operation,
        "kind": {"kind": kind},
        "name": name,
        "namespace": namespace,
        "userInfo": {"username": username, "groups": groups or []},
    }

    old = obj(annotations if old_annotations == "same" else old_annotations,
              labels if old_labels == "same" else old_labels)
    if operation == "DELETE":
        request["oldObject"] = old
    else:
        request["object"] = obj(annotations, labels)
        request["oldObject"] = old

    return DotMap({
        "binding": system_resources.BINDING_EDIT,
        "review": {"request": request},
        "snapshots": {
            system_resources.SNAP_ROLEBINDINGS: rolebindings or [],
            system_resources.SNAP_CLUSTERROLEBINDINGS: clusterrolebindings or [],
        },
    })


def exec_context(pod_name, *, namespace=NS, username=USER, groups=None, subresource="exec",
                 system_pods=None, rolebindings=None, clusterrolebindings=None):
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
        "snapshots": {
            system_resources.SNAP_SYSTEM_PODS: system_pods or [],
            system_resources.SNAP_ROLEBINDINGS: rolebindings or [],
            system_resources.SNAP_CLUSTERROLEBINDINGS: clusterrolebindings or [],
        },
    })


class TestSystemResourceEdit(unittest.TestCase):
    def run_hook(self, ctx):
        return hook.testrun(system_resources.main, [ctx])

    def test_system_resource_update_by_non_superadmin_is_denied(self):
        out = self.run_hook(edit_context("Pod", "dex-authenticator-0", annotations=SYSTEM_ANNOTATION))
        tests.assert_validation_deny(self, out, expected_system_deny("Pod", "dex-authenticator-0"))

    def test_system_resource_delete_by_non_superadmin_is_denied(self):
        out = self.run_hook(edit_context("PersistentVolumeClaim", "vm-disk", annotations=SYSTEM_ANNOTATION,
                                          operation="DELETE"))
        tests.assert_validation_deny(self, out, expected_system_deny("PersistentVolumeClaim", "vm-disk"))

    def test_system_resource_update_by_namespace_superadmin_is_allowed(self):
        out = self.run_hook(edit_context("Pod", "dex-authenticator-0", annotations=SYSTEM_ANNOTATION,
                                         rolebindings=superadmin_rolebinding()))
        tests.assert_validation_allowed(self, out, None)

    def test_system_resource_update_by_project_superadmin_is_allowed(self):
        rb = [rolebinding_snapshot(NS, "d8:project:superadmin", [user_subject(USER)])]
        out = self.run_hook(edit_context("Service", "managed-db", annotations=SYSTEM_ANNOTATION, rolebindings=rb))
        tests.assert_validation_allowed(self, out, None)

    def test_superadmin_via_clusterrolebinding_is_allowed(self):
        crb = [clusterrolebinding_snapshot("d8:project:superadmin", [user_subject(USER)])]
        out = self.run_hook(edit_context("Pod", "vm-pod", annotations=SYSTEM_ANNOTATION, clusterrolebindings=crb))
        tests.assert_validation_allowed(self, out, None)

    def test_superadmin_via_group_is_allowed(self):
        rb = [rolebinding_snapshot(NS, "d8:namespace:superadmin", [group_subject("platform-admins")])]
        out = self.run_hook(edit_context("Pod", "vm-pod", annotations=SYSTEM_ANNOTATION,
                                         groups=["platform-admins"], rolebindings=rb))
        tests.assert_validation_allowed(self, out, None)

    def test_superadmin_binding_in_other_namespace_does_not_apply(self):
        rb = [rolebinding_snapshot("other-ns", "d8:namespace:superadmin", [user_subject(USER)])]
        out = self.run_hook(edit_context("Pod", "dex-authenticator-0", annotations=SYSTEM_ANNOTATION, rolebindings=rb))
        tests.assert_validation_deny(self, out, expected_system_deny("Pod", "dex-authenticator-0"))

    def test_annotation_strip_in_same_update_is_still_denied(self):
        # User removes the annotation in the same UPDATE; old object still carries it -> deny.
        out = self.run_hook(edit_context("Pod", "dex-authenticator-0", annotations={},
                                         old_annotations=SYSTEM_ANNOTATION))
        tests.assert_validation_deny(self, out, expected_system_deny("Pod", "dex-authenticator-0"))

    def test_heritage_resource_mutation_by_non_superadmin_is_denied(self):
        out = self.run_hook(edit_context("ConfigMap", "tmpl-cm", labels=HERITAGE_LABELS))
        tests.assert_validation_deny(self, out, expected_heritage_deny("ConfigMap", "tmpl-cm"))

    def test_heritage_resource_mutation_even_by_superadmin_is_denied(self):
        out = self.run_hook(edit_context("ConfigMap", "tmpl-cm", labels=HERITAGE_LABELS,
                                         rolebindings=superadmin_rolebinding()))
        tests.assert_validation_deny(self, out, expected_heritage_deny("ConfigMap", "tmpl-cm"))

    def test_heritage_resource_mutation_by_cluster_admin_is_allowed(self):
        out = self.run_hook(edit_context("ConfigMap", "tmpl-cm", labels=HERITAGE_LABELS,
                                         groups=["system:masters"]))
        tests.assert_validation_allowed(self, out, None)

    def test_unmarked_resource_is_allowed(self):
        out = self.run_hook(edit_context("ConfigMap", "app-config"))
        tests.assert_validation_allowed(self, out, None)

    def test_deckhouse_sa_bypasses(self):
        out = self.run_hook(edit_context("Pod", "dex-authenticator-0", annotations=SYSTEM_ANNOTATION,
                                         username="system:serviceaccount:d8-system:deckhouse"))
        tests.assert_validation_allowed(self, out, None)

    def test_controller_sa_bypasses_heritage(self):
        out = self.run_hook(edit_context("ConfigMap", "tmpl-cm", labels=HERITAGE_LABELS,
                                         username="system:serviceaccount:d8-multitenancy-manager:multitenancy-manager"))
        tests.assert_validation_allowed(self, out, None)


class TestSystemResourceExec(unittest.TestCase):
    def run_hook(self, ctx):
        return hook.testrun(system_resources.main, [ctx])

    def test_exec_into_system_pod_by_non_superadmin_is_denied(self):
        out = self.run_hook(exec_context("dex-authenticator-0", system_pods=[pod_snapshot(NS, "dex-authenticator-0")]))
        tests.assert_validation_deny(self, out, expected_exec_deny("dex-authenticator-0", "exec"))

    def test_attach_into_system_pod_by_non_superadmin_is_denied(self):
        out = self.run_hook(exec_context("vm-pod", subresource="attach",
                                         system_pods=[pod_snapshot(NS, "vm-pod")]))
        tests.assert_validation_deny(self, out, expected_exec_deny("vm-pod", "attach"))

    def test_portforward_into_system_pod_by_non_superadmin_is_denied(self):
        out = self.run_hook(exec_context("vm-pod", subresource="portforward",
                                         system_pods=[pod_snapshot(NS, "vm-pod")]))
        tests.assert_validation_deny(self, out, expected_exec_deny("vm-pod", "portforward"))

    def test_exec_into_system_pod_by_superadmin_is_allowed(self):
        out = self.run_hook(exec_context("dex-authenticator-0",
                                         system_pods=[pod_snapshot(NS, "dex-authenticator-0")],
                                         rolebindings=superadmin_rolebinding()))
        tests.assert_validation_allowed(self, out, None)

    def test_exec_into_non_system_pod_is_allowed(self):
        out = self.run_hook(exec_context("my-app-0", system_pods=[pod_snapshot(NS, "dex-authenticator-0")]))
        tests.assert_validation_allowed(self, out, None)

    def test_exec_into_system_pod_in_other_namespace_is_allowed(self):
        # The snapshotted system pod is in another namespace; this request targets a different one.
        out = self.run_hook(exec_context("dex-authenticator-0", namespace="team-b",
                                         system_pods=[pod_snapshot(NS, "dex-authenticator-0")]))
        tests.assert_validation_allowed(self, out, None)

    def test_exec_by_cluster_admin_is_allowed(self):
        out = self.run_hook(exec_context("dex-authenticator-0", groups=["system:masters"],
                                         system_pods=[pod_snapshot(NS, "dex-authenticator-0")]))
        tests.assert_validation_allowed(self, out, None)


if __name__ == "__main__":
    unittest.main()
