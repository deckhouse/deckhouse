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
from pathlib import Path
from unittest import mock

from deckhouse import hook, tests
from deckhouse.tests import load_validation_webhook
from dotmap import DotMap

heritage = load_validation_webhook(Path(__file__).parent / "heritage.py")

NS = "team-a"
USER = "alice@example.com"
HERITAGE = {"heritage": "deckhouse"}


class FakeAPI:
    """Answers heritage._get from canned objects keyed by API path; a missing path is NotFound."""

    def __init__(self, objects=None, error=None):
        self.objects = objects or {}
        self.error = error
        self.paths = []

    def get(self, path):
        self.paths.append(path)
        if self.error:
            raise RuntimeError(self.error)
        return self.objects.get(path)


def obj(kind, labels=None):
    return {"kind": kind, "metadata": {"labels": labels} if labels is not None else {}}


def connect_context(subresource, pod="postgres-0", namespace=NS):
    return DotMap({"review": {"request": {
        "operation": "CONNECT",
        "kind": {"group": "", "version": "v1", "kind": "PodExecOptions"},
        "resource": {"group": "", "version": "v1", "resource": "pods"},
        "subResource": subresource,
        "name": pod,
        "namespace": namespace,
        "userInfo": {"username": USER, "groups": ["superadmins"]},
    }}})


def scale_context(group, version, resource, name, namespace=NS):
    return DotMap({"review": {"request": {
        "operation": "UPDATE",
        "kind": {"group": "autoscaling", "version": "v1", "kind": "Scale"},
        "resource": {"group": group, "version": version, "resource": resource},
        "subResource": "scale",
        "name": name,
        "namespace": namespace,
        "userInfo": {"username": USER, "groups": ["superadmins"]},
    }}})


def run(ctx, api):
    with mock.patch.object(heritage, "_get", api.get):
        return hook.testrun(heritage.main, [ctx])


POD_PATH = f"/api/v1/namespaces/{NS}/pods/postgres-0"


class TestConnect(unittest.TestCase):
    def test_every_connect_subresource_into_a_heritage_pod_is_denied(self):
        for subresource in ("exec", "attach", "portforward"):
            with self.subTest(subresource=subresource):
                api = FakeAPI({POD_PATH: obj("Pod", HERITAGE)})
                out = run(connect_context(subresource), api)
                tests.assert_validation_deny(
                    self, out,
                    f'{subresource} into Pod "postgres-0" in namespace "{NS}" is forbidden: the Pod '
                    "is labeled `heritage: deckhouse`, and only Deckhouse system users may exec, "
                    "attach or port-forward into the platform's own Pods.",
                )
                self.assertEqual(api.paths, [POD_PATH])

    def test_a_pod_without_the_label_is_allowed(self):
        out = run(connect_context("portforward"), FakeAPI({POD_PATH: obj("Pod", {"app": "postgres"})}))
        tests.assert_validation_allowed(self, out, None)

    def test_a_pod_without_labels_is_allowed(self):
        out = run(connect_context("exec"), FakeAPI({POD_PATH: obj("Pod")}))
        tests.assert_validation_allowed(self, out, None)

    def test_another_heritage_value_is_allowed(self):
        out = run(connect_context("exec"), FakeAPI({POD_PATH: obj("Pod", {"heritage": "multitenancy-manager"})}))
        tests.assert_validation_allowed(self, out, None)

    def test_a_pod_that_is_gone_is_allowed(self):
        out = run(connect_context("exec"), FakeAPI())
        tests.assert_validation_allowed(self, out, None)

    def test_maintenance_does_not_open_a_heritage_pod(self):
        labels = dict(HERITAGE, **{heritage.MAINTENANCE_LABEL: "true"})
        out = run(connect_context("portforward"), FakeAPI({POD_PATH: obj("Pod", labels)}))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_an_unreadable_pod_is_denied(self):
        out = run(connect_context("exec"), FakeAPI(error="connection refused"))
        tests.assert_validation_deny(self, out, "Internal webhook error: connection refused")


class TestScale(unittest.TestCase):
    def test_scaling_a_heritage_deployment_is_denied(self):
        path = f"/apis/apps/v1/namespaces/{NS}/deployments/d8ms-postgres"
        api = FakeAPI({path: obj("Deployment", HERITAGE)})
        out = run(scale_context("apps", "v1", "deployments", "d8ms-postgres"), api)
        tests.assert_validation_deny(
            self, out,
            f'Scaling deployments "d8ms-postgres" in namespace "{NS}" is forbidden: the Deployment '
            "is labeled `heritage: deckhouse`, and only Deckhouse system users may change the "
            "objects the platform manages.",
        )
        self.assertEqual(api.paths, [path])

    def test_scaling_a_heritage_statefulset_and_replicaset_is_denied(self):
        for resource, kind in (("statefulsets", "StatefulSet"), ("replicasets", "ReplicaSet")):
            with self.subTest(resource=resource):
                path = f"/apis/apps/v1/namespaces/{NS}/{resource}/db"
                out = run(scale_context("apps", "v1", resource, "db"), FakeAPI({path: obj(kind, HERITAGE)}))
                self.assertFalse(out.validations.data[0]["allowed"])

    def test_a_core_replication_controller_is_read_from_the_core_api(self):
        path = f"/api/v1/namespaces/{NS}/replicationcontrollers/legacy"
        api = FakeAPI({path: obj("ReplicationController", HERITAGE)})
        out = run(scale_context("", "v1", "replicationcontrollers", "legacy"), api)
        self.assertFalse(out.validations.data[0]["allowed"])
        self.assertEqual(api.paths, [path])

    def test_scaling_a_user_object_is_allowed(self):
        path = f"/apis/apps/v1/namespaces/{NS}/deployments/web"
        out = run(scale_context("apps", "v1", "deployments", "web"), FakeAPI({path: obj("Deployment", {"app": "web"})}))
        tests.assert_validation_allowed(self, out, None)

    def test_an_object_under_maintenance_is_the_users_to_scale(self):
        path = f"/apis/apps/v1/namespaces/{NS}/deployments/d8ms-postgres"
        for value in ("", "true"):
            with self.subTest(value=value):
                labels = dict(HERITAGE, **{heritage.MAINTENANCE_LABEL: value})
                out = run(scale_context("apps", "v1", "deployments", "d8ms-postgres"),
                          FakeAPI({path: obj("Deployment", labels)}))
                tests.assert_validation_allowed(self, out, None)

    def test_another_maintenance_value_does_not_open_the_object(self):
        path = f"/apis/apps/v1/namespaces/{NS}/deployments/d8ms-postgres"
        labels = dict(HERITAGE, **{heritage.MAINTENANCE_LABEL: "false"})
        out = run(scale_context("apps", "v1", "deployments", "d8ms-postgres"), FakeAPI({path: obj("Deployment", labels)}))
        self.assertFalse(out.validations.data[0]["allowed"])

    def test_an_unreadable_parent_is_denied(self):
        out = run(scale_context("apps", "v1", "deployments", "web"), FakeAPI(error="forbidden"))
        tests.assert_validation_deny(self, out, "Internal webhook error: forbidden")


class TestOtherRequests(unittest.TestCase):
    def test_a_request_the_rules_do_not_route_here_is_allowed_without_a_read(self):
        ctx = DotMap({"review": {"request": {
            "operation": "UPDATE",
            "kind": {"group": "", "version": "v1", "kind": "Pod"},
            "resource": {"group": "", "version": "v1", "resource": "pods"},
            "subResource": "status",
            "name": "postgres-0",
            "namespace": NS,
            "userInfo": {"username": USER},
        }}})
        api = FakeAPI()
        tests.assert_validation_allowed(self, run(ctx, api), None)
        self.assertEqual(api.paths, [])


class TestRead(unittest.TestCase):
    """_get tells a missing object from a failed read: the first has nothing to protect, the second
    must not be mistaken for it."""

    def _kubectl(self, returncode, stdout="", stderr=""):
        proc = mock.Mock(returncode=returncode, stdout=stdout, stderr=stderr)
        return mock.patch.object(heritage.subprocess, "run", return_value=proc)

    def test_an_object_is_parsed(self):
        with self._kubectl(0, stdout='{"kind": "Pod"}') as run_mock:
            self.assertEqual(heritage._get(POD_PATH), {"kind": "Pod"})
        self.assertEqual(run_mock.call_args.args[0], ["kubectl", "get", "--raw", POD_PATH])

    def test_no_client_config_flag_is_passed(self):
        # Any client-config flag makes kubectl skip its in-cluster config and dial localhost:8080
        # in the pod, so every read fails and the hook denies every request, labeled target or not.
        # The read is bounded by the subprocess timeout instead.
        with self._kubectl(0, stdout="{}") as run_mock:
            heritage._get(POD_PATH)
        cmd = run_mock.call_args.args[0]
        for flag in ("--request-timeout", "--server", "-s", "--kubeconfig", "--context", "--cluster",
                     "--user", "--token", "--insecure-skip-tls-verify", "--certificate-authority"):
            self.assertFalse(any(arg == flag or arg.startswith(flag + "=") for arg in cmd), flag)
        self.assertEqual(run_mock.call_args.kwargs["timeout"], heritage._API_TIMEOUT_SECONDS)

    def test_not_found_is_none(self):
        with self._kubectl(1, stderr='Error from server (NotFound): pods "postgres-0" not found'):
            self.assertIsNone(heritage._get(POD_PATH))

    def test_any_other_failure_raises(self):
        with self._kubectl(1, stderr="Error from server (Forbidden): pods is forbidden"):
            with self.assertRaises(RuntimeError):
                heritage._get(POD_PATH)


class TestObjectPath(unittest.TestCase):
    def test_names_are_escaped(self):
        self.assertEqual(
            heritage.object_path("apps", "v1", "deployments", "ns", "a/b"),
            "/apis/apps/v1/namespaces/ns/deployments/a%2Fb",
        )


if __name__ == "__main__":
    unittest.main()
