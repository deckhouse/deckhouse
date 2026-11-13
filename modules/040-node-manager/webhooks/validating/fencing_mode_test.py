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

fencing_mode = load_validation_webhook(Path(__file__).parent / "fencing_mode.py")

STATES_PATH = "/apis/node-manager.deckhouse.io/v1alpha1/fencingfailednodestates"


class FakeAPI:
    """Answers fencing_mode._get from the FencingFailedNodeStates and the groups of the Nodes."""

    def __init__(self, states=(), groups=None, error=None):
        self.states = list(states)
        self.groups = groups or {}
        self.error = error
        self.paths = []

    def get(self, path):
        self.paths.append(path)
        if self.error:
            raise RuntimeError(self.error)
        if path == STATES_PATH:
            return {"items": [{"metadata": {"name": name}} for name in self.states]}
        name = path.rsplit("/", 1)[-1]
        if path == f"/api/v1/nodes/{name}" and name in self.groups:
            return {"metadata": {"name": name, "labels": {"node.deckhouse.io/group": self.groups[name]}}}
        return None


GROUPS = {"worker-0": "worker", "worker-1": "worker", "other-0": "other"}


def node_group(mode, labels=None):
    spec = {"nodeType": "Static"}
    if mode is not None:
        spec["fencing"] = {"mode": mode}
    metadata = {"name": "worker"}
    if labels:
        metadata["labels"] = labels
    return {"apiVersion": "deckhouse.io/v1", "kind": "NodeGroup", "metadata": metadata, "spec": spec}


def update(old, new):
    return DotMap({"review": {"request": {
        "operation": "UPDATE",
        "name": "worker",
        "oldObject": old,
        "object": new,
    }}})


def run(ctx, api, community=False):
    with mock.patch.object(fencing_mode, "_get", api.get), \
            mock.patch.object(fencing_mode, "COMMUNITY_EDITION", community):
        return hook.testrun(fencing_mode.main, [ctx])


class TestModeChange(unittest.TestCase):
    def assert_denied_for(self, out, nodes):
        self.assertFalse(out.validations.data[0]["allowed"])
        message = out.validations.data[0]["message"]
        self.assertIn("FencingFailedNodeState: " + ", ".join(nodes) + ".", message)
        self.assertIn("An incident ends when the node comes back after a reboot or when its Node object is deleted", message)
        self.assertNotIn("other-0", message)

    def test_a_mode_change_with_states_of_group_nodes_is_denied(self):
        api = FakeAPI(states=["worker-1", "other-0", "worker-0"], groups=GROUPS)
        out = run(update(node_group("Normal"), node_group("Slow")), api)
        self.assert_denied_for(out, ["worker-0", "worker-1"])

    def test_turning_off_with_a_state_is_denied(self):
        out = run(update(node_group("Normal"), node_group("Disabled")), FakeAPI(states=["worker-1"], groups=GROUPS))
        self.assert_denied_for(out, ["worker-1"])

    def test_turning_on_with_a_state_is_denied(self):
        out = run(update(node_group(None), node_group("Normal")), FakeAPI(states=["worker-0"], groups=GROUPS))
        self.assert_denied_for(out, ["worker-0"])

    def test_a_state_of_another_group_does_not_block(self):
        out = run(update(node_group("Normal"), node_group("Slow")), FakeAPI(states=["other-0"], groups=GROUPS))
        tests.assert_validation_allowed(self, out, None)

    def test_a_state_of_a_deleted_node_does_not_block(self):
        out = run(update(node_group("Normal"), node_group("Slow")), FakeAPI(states=["gone-0"], groups=GROUPS))
        tests.assert_validation_allowed(self, out, None)

    def test_a_mode_change_without_states_is_allowed(self):
        out = run(update(node_group("Normal"), node_group("Slow")), FakeAPI(groups=GROUPS))
        tests.assert_validation_allowed(self, out, None)

    def test_the_critical_mode_in_the_commercial_editions_is_not_normal(self):
        out = run(update(node_group("Critical"), node_group("Normal")), FakeAPI(states=["worker-0"], groups=GROUPS))
        self.assert_denied_for(out, ["worker-0"])

    def test_an_unreadable_state_list_denies_a_mode_change(self):
        out = run(update(node_group("Normal"), node_group("Slow")), FakeAPI(error="connection refused"))
        tests.assert_validation_deny(self, out, "Internal webhook error: connection refused")


class TestSameMode(unittest.TestCase):
    """An edit that keeps the mode after the normalization is allowed and reads nothing."""

    def assert_allowed_without_reads(self, old, new, community=False):
        api = FakeAPI(states=["worker-0"], groups=GROUPS, error="must not be read")
        tests.assert_validation_allowed(self, run(update(old, new), api, community), None)
        self.assertEqual(api.paths, [])

    def test_another_field(self):
        self.assert_allowed_without_reads(node_group("Normal"), node_group("Normal", labels={"team": "a"}))

    def test_watchdog_is_slow(self):
        self.assert_allowed_without_reads(node_group("Watchdog"), node_group("Slow"))

    def test_no_fencing_is_disabled(self):
        self.assert_allowed_without_reads(node_group(None), node_group("Disabled"))

    def test_notify_and_unknown_values_are_disabled(self):
        self.assert_allowed_without_reads(node_group("Notify"), node_group("Disabled"))
        self.assert_allowed_without_reads(node_group("off"), node_group(None))

    def test_critical_is_normal_in_community_edition(self):
        self.assert_allowed_without_reads(node_group("Critical"), node_group("Normal"), community=True)


class TestRead(unittest.TestCase):
    """_get tells a deleted Node from a failed read: the first does not block, the second denies."""

    def _kubectl(self, returncode, stdout="", stderr=""):
        proc = mock.Mock(returncode=returncode, stdout=stdout, stderr=stderr)
        return mock.patch.object(fencing_mode.subprocess, "run", return_value=proc)

    def test_an_object_is_parsed(self):
        with self._kubectl(0, stdout='{"items": []}') as run_mock:
            self.assertEqual(fencing_mode._get(STATES_PATH), {"items": []})
        self.assertEqual(run_mock.call_args.args[0], ["kubectl", "get", "--raw", STATES_PATH])

    def test_not_found_is_none(self):
        with self._kubectl(1, stderr='Error from server (NotFound): nodes "worker-0" not found'):
            self.assertIsNone(fencing_mode._get("/api/v1/nodes/worker-0"))

    def test_any_other_failure_raises(self):
        with self._kubectl(1, stderr="Error from server (Forbidden): nodes is forbidden"):
            with self.assertRaises(RuntimeError):
                fencing_mode._get("/api/v1/nodes/worker-0")


if __name__ == "__main__":
    unittest.main()
