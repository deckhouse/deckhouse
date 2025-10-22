#!/usr/bin/python3
import json
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

import unittest
from instance_class import main
from deckhouse import hook, tests
from dotmap import DotMap
import json  # Ensure json is imported for loading binding context

def _prepare_delete_instance_class_binding_context(cloud_name: str, has_node_groups: bool) -> DotMap:
    kind_name = f"{cloud_name}InstanceClass" if cloud_name else "InstanceClass"
    binding_context_json = f"""
    {{
      "binding": "instanceclass-in-nodegroups.deckhouse.io",
      "review": {{
        "request": {{
          "uid": "d47e6935-8e58-4270-b193-c4a8e2626ba1",
          "kind": {{
            "group": "deckhouse.io",
            "version": "v1",
            "kind": "{kind_name}"
          }},
          "resource": {{
            "group": "deckhouse.io",
            "version": "v",
            "resource": "{cloud_name.lower()}instanceclasses"
          }},
          "requestKind": {{
            "group": "deckhouse.io",
            "version": "v1",
            "kind": "{kind_name}"
          }},
          "requestResource": {{
            "group": "deckhouse.io",
            "version": "v1",
            "resource": "{cloud_name.lower()}instanceclasses"
          }},
          "name": "worker-test",
          "operation": "DELETE",
          "userInfo": {{
            "username": "kubernetes-admin",
            "groups": [
              "system:masters",
              "system:authenticated"
            ]
          }},
          "object": null,
          "oldObject": {{
            "apiVersion": "deckhouse.io/v1alpha1",
            "kind": "{kind_name}",
            "metadata": {{
              "creationTimestamp": "2024-11-22T08:00:33Z",
              "generation": 1,
              "managedFields": [
                {{
                  "apiVersion": "deckhouse.io/v1",
                  "fieldsType": "FieldsV1",
                  "fieldsV1": {{
                    "f:spec": {{
                      ".": {{}},
                      "f:coreFraction": {{}},
                      "f:cores": {{}},
                      "f:diskSizeGB": {{}},
                      "f:imageID": {{}},
                      "f:memory": {{}},
                      "f:networkType": {{}}
                    }}
                  }},
                  "manager": "kubectl-client-side-apply",
                  "operation": "Update",
                  "time": "2025-02-17T16:01:42Z"
                }}
              ],
              "name": "worker-test",
              "resourceVersion": "7511300",
              "uid": "92ce2620-847d-4e45-aaa0-c0e314b33142"
            }},
            "spec": {{
              "coreFraction": 20,
              "cores": 2,
              "diskSizeGB": 20,
              "imageID": "fd8s25j1obln4fsn1qor",
              "memory": 8192,
              "networkType": "Standard"
            }},
            "status": {{
                "nodeGroupConsumers": {json.dumps(["nodegroup1", "nodegroup2"] if has_node_groups else [])}
            }}
          }},
          "dryRun": false,
          "options": {{
            "kind": "DeleteOptions",
            "apiVersion": "meta.k8s.io/v1",
            "propagationPolicy": "Background"
          }}
        }}
      }},
      "type": "Validating"
    }}
    """
    ctx_dict = json.loads(binding_context_json)
    return DotMap(ctx_dict)

class TestInstanceClassValidationWebhook(unittest.TestCase):
    def test_should_allow_delete_when_no_nodegroup_consumers(self):
        for cloud_name in ["Yandex", "AWS", "GCP", "Azure", "Openstack"]:
            with self.subTest(cloud=cloud_name):
                ctx = _prepare_delete_instance_class_binding_context(cloud_name, False)
                out = hook.testrun(main, [ctx])
                tests.assert_validation_allowed(self, out, None)

    def test_should_deny_delete_when_has_nodegroup_consumers(self):
        for cloud_name in ["Yandex", "AWS", "GCP", "Azure", "Openstack"]:
            with self.subTest(cloud=cloud_name):
                ctx = _prepare_delete_instance_class_binding_context(cloud_name, True)
                out = hook.testrun(main, [ctx])
                expected_error = f"{cloud_name}InstanceClass/worker-test cannot be deleted because it is being used by NodeGroup: nodegroup1, nodegroup2"
                tests.assert_validation_deny(self, out, expected_error)

    def test_should_skip_generic_instance_class(self):
        ctx = _prepare_delete_instance_class_binding_context("", False)
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)


if __name__ == '__main__':
    unittest.main()
