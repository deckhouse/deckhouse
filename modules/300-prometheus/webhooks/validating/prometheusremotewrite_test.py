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
from prometheusremotewrite import main
from deckhouse import hook, tests
from dotmap import DotMap
import json  # Ensure json is imported for loading binding context

def _prepare_validation_binding_context(binding_context_json, new_spec: dict) -> DotMap:
    ctx_dict = json.loads(binding_context_json)
    ctx = DotMap(ctx_dict)
    ctx.review.request.object.spec = new_spec
    return ctx

def _prepare_prometheusremotewrites_class_binding_context(new_spec: dict) -> DotMap:
    binding_context_json = f"""
    {{
      "binding": "prometheusremotewrites.deckhouse.io",
      "review": {{
        "request": {{
          "uid": "d47e6935-8e58-4270-b193-c4a8e2626ba1",
          "kind": {{
            "group": "deckhouse.io",
            "version": "v1",
            "kind": "Prometheusremotewrites"
          }},
          "resource": {{
            "group": "deckhouse.io",
            "version": "v",
            "resources": "prometheusremotewrites"
          }},
          "requestKind": {{
            "group": "deckhouse.io",
            "version": "v1",
            "kind": "Prometheusremotewrites"
          }},
          "requestResource": {{
            "group": "deckhouse.io",
            "version": "v1",
            "resource": "prometheusremotewrites"
          }},
          "name": "new",
          "operation": "UPDATE",
          "userInfo": {{
            "username": "kubernetes-admin",
            "groups": [
              "system:masters",
              "system:authenticated"
            ]
          }},
          "object": {{
            "apiVersion": "deckhouse.io/v1alpha1",
            "kind": "Prometheusremotewrites",
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
                      "f:url": {{}},
                      "f:tlsConfig": {{
                         "f:ca": {{}}
                      }}
                    }}
                  }},
                  "manager": "deckhouse-controller",
                  "operation": "Update",
                  "time": "2025-02-17T16:01:42Z"
                }}
              ],
              "name": "candi-admins",
              "resourceVersion": "7511300",
              "uid": "92ce2620-847d-4e45-aaa0-c0e314b33142"
            }},
            "spec": {{
              "url": "test"
            }}
          }},
          "oldObject": {{
            "apiVersion": "deckhouse.io/v1alpha1",
            "kind": "Prometheusremotewrites",
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
                      "f:url": {{}},
                      "f:tlsConfig": {{
                         "f:ca": {{}}
                      }}
                    }}
                  }},
                  "manager": "deckhouse-controller",
                  "operation": "Update",
                  "time": "2025-02-17T16:01:42Z"
                }}
              ],
              "name": "old",
              "resourceVersion": "7511300",
              "uid": "92ce2620-847d-4e45-aaa0-c0e314b33142"
            }},
            "spec": {{
              "url": "https://old.local",
              "tlsConfig": {{
                  "ca": "111"
              }}
            }}
          }},
          "dryRun": false,
          "options": {{
            "kind": "UpdateOptions",
            "apiVersion": "meta.k8s.io/v1",
            "propagationPolicy": "Background"
          }}
        }}
      }},
    "snapshots": {{
        "prometheusremotewrites": [
            {{
                "filterResult": {{
                    "name": "test_double",
                    "url": "https://test.local"
                }}
            }},
            {{
                "filterResult": {{
                    "name": "new",
                    "url": "https://new.local"
                }}
            }}
        ]
    }},
    "type": "Validating"
    }}
    """
    return _prepare_validation_binding_context(binding_context_json, new_spec)

class TestInstanceClassValidationWebhook(unittest.TestCase):
    def test_should_allow_update_url(self):
        ctx = _prepare_prometheusremotewrites_class_binding_context({
            "url": "https://new.local",
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_should_deny_double_url(self):
        ctx = _prepare_prometheusremotewrites_class_binding_context({
            "url": "https://test.local",
        })
        out = hook.testrun(main, [ctx])
        expected_error = "Remote write URL https://test.local is already in use"
        tests.assert_validation_deny(self, out, expected_error)

    def test_should_allow_update_valide_ca(self):
        ctx = _prepare_prometheusremotewrites_class_binding_context({
            "url": "https://new.local",
            "tlsConfig": {
                "ca": "-----BEGIN CERTIFICATE-----\nMIICCTCCAY6gAwIBAgINAgPluILrIPglJ209ZjAKBggqhkjOPQQDAzBHMQswCQYD\nVQQGEwJVUzEiMCAGA1UEChMZR29vZ2xlIFRydXN0IFNlcnZpY2VzIExMQzEUMBIG\nA1UEAxMLR1RTIFJvb3QgUjMwHhcNMTYwNjIyMDAwMDAwWhcNMzYwNjIyMDAwMDAw\nWjBHMQswCQYDVQQGEwJVUzEiMCAGA1UEChMZR29vZ2xlIFRydXN0IFNlcnZpY2Vz\nIExMQzEUMBIGA1UEAxMLR1RTIFJvb3QgUjMwdjAQBgcqhkjOPQIBBgUrgQQAIgNi\nAAQfTzOHMymKoYTey8chWEGJ6ladK0uFxh1MJ7x/JlFyb+Kf1qPKzEUURout736G\njOyxfi//qXGdGIRFBEFVbivqJn+7kAHjSxm65FSWRQmx1WyRRK2EE46ajA2ADDL2\n4CejQjBAMA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQW\nBBTB8Sa6oC2uhYHP0/EqEr24Cmf9vDAKBggqhkjOPQQDAwNpADBmAjEA9uEglRR7\nVKOQFhG/hMjqb2sXnh5GmCCbn9MN2azTL818+FsuVbu/3ZL3pAzcMeGiAjEA/Jdm\nZuVDFhOD3cffL74UOO0BzrEXGhF16b0DjyZ+hOXJYKaV11RZt+cRLInUue4X\n-----END CERTIFICATE-----"
            },
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_should_deny_invalid_ca(self):
        ctx = _prepare_prometheusremotewrites_class_binding_context({
            "url": "https://new.local",
            "tlsConfig": {
                "ca": "1111"
            },
        })
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(self, out, out.validations._data[0]["message"])

if __name__ == '__main__':
    unittest.main()
