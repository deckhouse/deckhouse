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

import copy
import unittest

from deckhouse import hook, tests

from pathlib import Path
from deckhouse.tests import load_validation_webhook

# The hook lives in a ValidationWebhook manifest; the helper assembles it the way webhook-operator does.
main = load_validation_webhook(Path(__file__).parent / "d8_secret_encryption_key_configuration.py").main

DECKHOUSE_SA = "system:serviceaccount:d8-system:deckhouse"
DELETE_MSG = "it is forbidden to delete secret d8-secret-encryption-key"
MODIFY_MSG = (
    "it is forbidden to modify secret d8-secret-encryption-key "
    "(only labels and annotations are allowed to be changed)"
)
USER_MSG = (
    "only system:serviceaccount:d8-system:deckhouse is allowed to modify labels and "
    "annotations of secret d8-secret-encryption-key"
)

SECRET = {
    "apiVersion": "v1",
    "kind": "Secret",
    "metadata": {
        "name": "d8-secret-encryption-key",
        "namespace": "kube-system",
        "labels": {"name": "d8-secret-encryption-key"},
        "annotations": {"a": "b"},
        "resourceVersion": "100",
        "managedFields": [{"manager": "deckhouse"}],
    },
    "data": {"secretbox.key": "c2VjcmV0"},
}


def ctx(operation, obj=None, old=None, user=DECKHOUSE_SA):
    request = {"operation": operation, "userInfo": {"username": user}}
    if obj is not None:
        request["object"] = obj
    if old is not None:
        request["oldObject"] = old
    return {
        "binding": "d8-secret-encryption-key-secret.deckhouse.io",
        "type": "Validating",
        "review": {"request": request},
    }


def changed(**metadata):
    obj = copy.deepcopy(SECRET)
    obj["metadata"].update(metadata)
    return obj


def run(binding_context):
    return hook.testrun(main, [binding_context])


class TestDelete(unittest.TestCase):
    def test_delete_denied_for_deckhouse(self):
        tests.assert_validation_deny(self, run(ctx("DELETE", old=SECRET)), DELETE_MSG)

    def test_delete_denied_for_anyone(self):
        tests.assert_validation_deny(self, run(ctx("DELETE", old=SECRET, user="admin")), DELETE_MSG)

    def test_delete_with_trailing_newline_like_bash(self):
        # bash command substitution strips trailing newlines from jq -r output
        tests.assert_validation_deny(self, run(ctx("DELETE\n\n")), DELETE_MSG)


class TestCreate(unittest.TestCase):
    def test_create_allowed_for_anyone(self):
        tests.assert_validation_allowed(self, run(ctx("CREATE", obj=SECRET, user="admin")), None)

    def test_connect_allowed(self):
        tests.assert_validation_allowed(self, run(ctx("CONNECT")), None)

    def test_missing_operation_allowed(self):
        tests.assert_validation_allowed(self, run({"review": {"request": {}}}), None)

    def test_missing_review_allowed(self):
        tests.assert_validation_allowed(self, run({}), None)

    def test_lowercase_delete_allowed(self):
        tests.assert_validation_allowed(self, run(ctx("delete")), None)


class TestUpdate(unittest.TestCase):
    def test_no_changes_by_deckhouse_allowed(self):
        tests.assert_validation_allowed(self, run(ctx("UPDATE", SECRET, SECRET)), None)

    def test_labels_annotations_by_deckhouse_allowed(self):
        new = changed(labels={"x": "y"}, annotations=None, resourceVersion="101", managedFields=[])
        tests.assert_validation_allowed(self, run(ctx("UPDATE", new, SECRET)), None)

    def test_removed_labels_by_deckhouse_allowed(self):
        new = copy.deepcopy(SECRET)
        del new["metadata"]["labels"], new["metadata"]["annotations"]
        tests.assert_validation_allowed(self, run(ctx("UPDATE", new, SECRET)), None)

    def test_labels_by_other_user_denied(self):
        new = changed(labels={"x": "y"})
        tests.assert_validation_deny(self, run(ctx("UPDATE", new, SECRET, user="admin")), USER_MSG)

    def test_no_changes_by_other_user_denied(self):
        tests.assert_validation_deny(self, run(ctx("UPDATE", SECRET, SECRET, user="admin")), USER_MSG)

    def test_missing_user_denied(self):
        tests.assert_validation_deny(self, run(ctx("UPDATE", SECRET, SECRET, user=None)), USER_MSG)

    def test_user_with_trailing_newline_allowed_like_bash(self):
        tests.assert_validation_allowed(self, run(ctx("UPDATE", SECRET, SECRET, user=DECKHOUSE_SA + "\n")), None)

    def test_data_change_denied(self):
        new = copy.deepcopy(SECRET)
        new["data"]["secretbox.key"] = "b3RoZXI="
        tests.assert_validation_deny(self, run(ctx("UPDATE", new, SECRET)), MODIFY_MSG)

    def test_name_change_denied(self):
        tests.assert_validation_deny(self, run(ctx("UPDATE", changed(name="other"), SECRET)), MODIFY_MSG)

    def test_new_metadata_field_denied(self):
        tests.assert_validation_deny(self, run(ctx("UPDATE", changed(finalizers=["x"]), SECRET)), MODIFY_MSG)

    def test_modify_checked_before_user(self):
        new = changed(name="other")
        tests.assert_validation_deny(self, run(ctx("UPDATE", new, SECRET, user="admin")), MODIFY_MSG)

    def test_missing_old_object_denied(self):
        tests.assert_validation_deny(self, run(ctx("UPDATE", SECRET)), MODIFY_MSG)

    def test_both_objects_missing_allowed(self):
        tests.assert_validation_allowed(self, run(ctx("UPDATE")), None)

    def test_null_metadata_equal(self):
        obj = {"kind": "Secret", "metadata": None}
        tests.assert_validation_allowed(self, run(ctx("UPDATE", obj, obj)), None)

    def test_bool_differs_from_number(self):
        new = dict(SECRET, immutable=True)
        old = dict(SECRET, immutable=1)
        tests.assert_validation_deny(self, run(ctx("UPDATE", new, old)), MODIFY_MSG)

    def test_int_equals_float(self):
        new = dict(SECRET, x=1)
        old = dict(SECRET, x=1.0)
        tests.assert_validation_allowed(self, run(ctx("UPDATE", new, old)), None)


class TestErrors(unittest.TestCase):
    """Inputs where the bash hook failed in jq; the template's main denies them with an internal error."""

    def assert_error(self, out, error):
        self.assertEqual(len(out.validations.data), 1)
        response = out.validations.data[0]
        self.assertFalse(response["allowed"])
        self.assertIn(f"Internal webhook error: {error}", response.values())

    def test_request_not_object(self):
        self.assert_error(run({"review": {"request": "x"}}), "cannot index str with 'operation'")

    def test_user_info_not_object(self):
        out = run({"review": {"request": {"operation": "DELETE", "userInfo": []}}})
        self.assert_error(out, "cannot index list with 'username'")

    def test_object_not_object(self):
        self.assert_error(run(ctx("UPDATE", "secret", SECRET)), "cannot index str with 'metadata'")

    def test_old_metadata_not_object(self):
        out = run(ctx("UPDATE", SECRET, {"metadata": "x"}))
        self.assert_error(out, "cannot delete fields of str metadata")


if __name__ == "__main__":
    unittest.main()
