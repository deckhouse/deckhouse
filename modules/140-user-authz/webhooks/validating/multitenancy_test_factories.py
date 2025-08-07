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

from typing import Optional

def prepare_car_binding_context(
        car_restricted_multitenancy_fields: bool,
        module_enable_multitenancy_field: Optional[bool]) -> str:
    return f"""
    {{
      "binding": "d8-user-authz-car-multitenancy-related-options.deckhouse.io",
      "review": {{
        "request": {{
          "uid": "2af480b8-c341-487a-8bfa-f9f0582d508f",
          "kind": {{
            "group": "deckhouse.io",
            "version": "v1",
            "kind": "ClusterAuthorizationRule"
          }},
          "resource": {{
            "group": "deckhouse.io",
            "version": "v1",
            "resource": "clusterauthorizationrules"
          }},
          "requestKind": {{
            "group": "deckhouse.io",
            "version": "v1",
            "kind": "ClusterAuthorizationRule"
          }},
          "requestResource": {{
            "group": "deckhouse.io",
            "version": "v1",
            "resource": "clusterauthorizationrules"
          }},
          "name": "user1",
          "operation": "UPDATE",
          "userInfo": {{
            "username": "kubernetes-admin",
            "groups": [
              "kubeadm:cluster-admins",
              "system:authenticated"
            ]
          }},
          "object": {{
            "apiVersion": "deckhouse.io/v1",
            "kind": "ClusterAuthorizationRule",
            "metadata": {{
              "annotations": {{}},
              "creationTimestamp": "2025-07-29T14:39:41Z",
              "generation": 7,
              "managedFields": [
                {{
                  "apiVersion": "deckhouse.io/v1",
                  "fieldsType": "FieldsV1",
                  "fieldsV1": {{
                    "f:metadata": {{
                      "f:annotations": {{
                        ".": {{}},
                        "f:kubectl.kubernetes.io/last-applied-configuration": {{}}
                      }}
                    }},
                    "f:spec": {{
                      ".": {{}},
                      "f:accessLevel": {{}},
                      "f:allowAccessToSystemNamespaces": {{}},
                      "f:allowScale": {{}},
                      "f:portForwarding": {{}},
                      "f:subjects": {{}}
                    }}
                  }},
                  "manager": "kubectl-client-side-apply",
                  "operation": "Update",
                  "time": "2025-07-29T23:37:23Z"
                }}
              ],
              "name": "user1",
              "resourceVersion": "663739",
              "uid": "a95cbb12-8685-4dbf-a8ca-92c922617976"
            }},
            "spec": {{
              "accessLevel": "PrivilegedUser",
              {'''"allowAccessToSystemNamespaces": true,
              "limitNamespaces": ["production-*"],
              "namespaceSelector": {"matchLabels": {"env": "prod"}},''' if car_restricted_multitenancy_fields else ""
              }
              "allowScale": false,
              "portForwarding": true,
              "subjects": [
                {{
                  "kind": "User",
                  "name": "myuser"
                }}
              ]
            }}
          }},
          "oldObject": {{
            "apiVersion": "deckhouse.io/v1",
            "kind": "ClusterAuthorizationRule",
            "metadata": {{
              "annotations": {{}},
              "creationTimestamp": "2025-07-29T14:39:41Z",
              "generation": 6,
              "managedFields": [
                {{
                  "apiVersion": "deckhouse.io/v1",
                  "fieldsType": "FieldsV1",
                  "fieldsV1": {{
                    "f:metadata": {{
                      "f:annotations": {{
                        ".": {{}},
                        "f:kubectl.kubernetes.io/last-applied-configuration": {{}}
                      }}
                    }},
                    "f:spec": {{
                      ".": {{}},
                      "f:accessLevel": {{}},
                      "f:allowScale": {{}},
                      "f:portForwarding": {{}},
                      "f:subjects": {{}}
                    }}
                  }},
                  "manager": "kubectl-client-side-apply",
                  "operation": "Update",
                  "time": "2025-07-29T18:13:49Z"
                }}
              ],
              "name": "user1",
              "resourceVersion": "663739",
              "uid": "a95cbb12-8685-4dbf-a8ca-92c922617976"
            }},
            "spec": {{
              "accessLevel": "PrivilegedUser",
              "allowScale": false,
              "portForwarding": true,
              "subjects": [
                {{
                  "kind": "User",
                  "name": "myuser"
                }}
              ]
            }}
          }},
          "dryRun": false,
          "options": {{
            "kind": "UpdateOptions",
            "apiVersion": "meta.k8s.io/v1",
            "fieldManager": "kubectl-client-side-apply",
            "fieldValidation": "Strict"
          }}
        }}
      }},
      "snapshots": {{
        "d8-user-authz-moduleconfig": [
          {{
            "object": {{
              "apiVersion": "deckhouse.io/v1alpha1",
              "kind": "ModuleConfig",
              "metadata": {{
                "creationTimestamp": "2025-07-29T02:01:51Z",
                "finalizers": [
                  "modules.deckhouse.io/module-registered"
                ],
                "generation": 19,
                "managedFields": [
                  {{
                    "apiVersion": "deckhouse.io/v1alpha1",
                    "fieldsType": "FieldsV1",
                    "fieldsV1": {{
                      "f:spec": {{
                        ".": {{}},
                        "f:enabled": {{}},
                        "f:version": {{}}
                      }}
                    }},
                    "manager": "dhctl",
                    "operation": "Update",
                    "time": "2025-07-29T02:01:51Z"
                  }},
                  {{
                    "apiVersion": "deckhouse.io/v1alpha1",
                    "fieldsType": "FieldsV1",
                    "fieldsV1": {{
                      "f:metadata": {{
                        "f:finalizers": {{
                          ".": {{}}
                        }}
                      }}
                    }},
                    "manager": "deckhouse-controller",
                    "operation": "Update",
                    "time": "2025-07-29T02:02:28Z"
                  }},
                  {{
                    "apiVersion": "deckhouse.io/v1alpha1",
                    "fieldsType": "FieldsV1",
                    "fieldsV1": {{
                      "f:status": {{
                        ".": {{}},
                        "f:message": {{}},
                        "f:version": {{}}
                      }}
                    }},
                    "manager": "deckhouse-controller",
                    "operation": "Update",
                    "subresource": "status",
                    "time": "2025-07-29T02:02:28Z"
                  }},
                  {{
                    "apiVersion": "deckhouse.io/v1alpha1",
                    "fieldsType": "FieldsV1",
                    "fieldsV1": {{
                      "f:spec": {{
                        "f:settings": {{
                          ".": {{}},
                          "f:enableMultiTenancy": {{}}
                        }}
                      }}
                    }},
                    "manager": "kubectl-edit",
                    "operation": "Update",
                    "time": "2025-07-29T23:27:00Z"
                  }}
                ],
                "name": "user-authz",
                "resourceVersion": "663947",
                "uid": "71324cad-b74b-45ce-b122-1040558471ee"
              }},
              "spec": {{
                "enabled": true,
                "settings": {{
                  {'' if module_enable_multitenancy_field is None else ('"enableMultiTenancy": true' if module_enable_multitenancy_field else '"enableMultiTenancy": false')}
                }},
                "version": 1
              }},
              "status": {{
                "message": "",
                "version": "1"
              }}
            }}
          }}
        ]
      }},
      "type": "Validating"
    }}
    """

class CAR:
    name: str
    include_multitenancy_related_fields: bool = True

    def __init__(self, name: str, include_multitenancy_related_fields: bool = True):
        self.name = name
        self.include_multitenancy_related_fields = include_multitenancy_related_fields

    def toSnapshotObject(self) -> str:
        return f"""
        {{
          "object": {{
            "apiVersion": "deckhouse.io/v1alpha1",
            "kind": "ClusterAuthorizationRule",
            "metadata": {{
              "annotations": {{}},
              "creationTimestamp": "2025-07-29T14:39:41Z",
              "generation": 6,
              "managedFields": [
                {{
                  "apiVersion": "deckhouse.io/v1",
                  "fieldsType": "FieldsV1",
                  "fieldsV1": {{
                    "f:metadata": {{
                      "f:annotations": {{
                        ".": {{}},
                        "f:kubectl.kubernetes.io/last-applied-configuration": {{}}
                      }}
                    }},
                    "f:spec": {{
                      ".": {{}},
                      "f:accessLevel": {{}},
                      "f:allowScale": {{}},
                      "f:portForwarding": {{}},
                      "f:subjects": {{}}
                    }}
                  }},
                  "manager": "kubectl-client-side-apply",
                  "operation": "Update",
                  "time": "2025-07-29T18:13:49Z"
                }}
              ],
              "name": "{self.name}",
              "resourceVersion": "663739",
              "uid": "a95cbb12-8685-4dbf-a8ca-92c922617976"
            }},
            "spec": {{
              "accessLevel": "PrivilegedUser",
              "allowScale": true,
              "portForwarding": true,
              {'''"allowAccessToSystemNamespaces": true,
              "limitNamespaces": ["production-*"],
              "namespaceSelector": {"matchLabels": {"env": "prod"}},''' if self.include_multitenancy_related_fields else ""
              }
              "subjects": [
                {{
                  "kind": "User",
                  "name": "{self.name}"
                }}
              ]
            }}
          }}
        }}"""

def build_three_mixed_multitenancy_related_and_not_related_cars() -> list[CAR]:
    """
    Builds three ClusterAuthorizationRule objects with a mix of multitenancy-related and non-related fields.
    """
    return [
        CAR(name="user1", include_multitenancy_related_fields=True),
        CAR(name="user2", include_multitenancy_related_fields=False),
        CAR(name="user3", include_multitenancy_related_fields=True)
    ]

def build_three_not_multitenancy_related_cars() -> list[CAR]:
    """
    Builds three ClusterAuthorizationRule objects without multitenancy-related fields.
    """
    return [
        CAR(name="user1", include_multitenancy_related_fields=False),
        CAR(name="user2", include_multitenancy_related_fields=False),
        CAR(name="user3", include_multitenancy_related_fields=False)
    ]


def prepare_module_config_binding_context(module_enable_multitenancy_field: Optional[bool], cars: list[CAR] = []) -> str:
    cars_snapshot = ','.join(car.toSnapshotObject() for car in cars)
    
    return f"""
{{
  "binding": "d8-user-authz-module-multitenancy-related-options.deckhouse.io",
  "review": {{
    "request": {{
      "uid": "801af911-366a-4ef0-ab0e-249a3de40827",
      "kind": {{
        "group": "deckhouse.io",
        "version": "v1alpha1",
        "kind": "ModuleConfig"
      }},
      "resource": {{
        "group": "deckhouse.io",
        "version": "v1alpha1",
        "resource": "moduleconfigs"
      }},
      "requestKind": {{
        "group": "deckhouse.io",
        "version": "v1alpha1",
        "kind": "ModuleConfig"
      }},
      "requestResource": {{
        "group": "deckhouse.io",
        "version": "v1alpha1",
        "resource": "moduleconfigs"
      }},
      "name": "user-authz",
      "operation": "UPDATE",
      "userInfo": {{
        "username": "kubernetes-admin",
        "groups": [
          "kubeadm:cluster-admins",
          "system:authenticated"
        ]
      }},
      "object": {{
        "apiVersion": "deckhouse.io/v1alpha1",
        "kind": "ModuleConfig",
        "metadata": {{
          "creationTimestamp": "2025-07-29T02:01:51Z",
          "finalizers": [
            "modules.deckhouse.io/module-registered"
          ],
          "generation": 20,
          "managedFields": [
            {{
              "apiVersion": "deckhouse.io/v1alpha1",
              "fieldsType": "FieldsV1",
              "fieldsV1": {{
                "f:spec": {{
                  ".": {{}},
                  "f:enabled": {{}},
                  "f:version": {{}}
                }}
              }},
              "manager": "dhctl",
              "operation": "Update",
              "time": "2025-07-29T02:01:51Z"
            }},
            {{
              "apiVersion": "deckhouse.io/v1alpha1",
              "fieldsType": "FieldsV1",
              "fieldsV1": {{
                "f:metadata": {{
                  "f:finalizers": {{
                    ".": {{}}
                  }}
                }}
              }},
              "manager": "deckhouse-controller",
              "operation": "Update",
              "time": "2025-07-29T02:02:28Z"
            }},
            {{
              "apiVersion": "deckhouse.io/v1alpha1",
              "fieldsType": "FieldsV1",
              "fieldsV1": {{
                "f:status": {{
                  ".": {{}},
                  "f:message": {{}},
                  "f:version": {{}}
                }}
              }},
              "manager": "deckhouse-controller",
              "operation": "Update",
              "subresource": "status",
              "time": "2025-07-29T02:02:28Z"
            }},
            {{
              "apiVersion": "deckhouse.io/v1alpha1",
              "fieldsType": "FieldsV1",
              "fieldsV1": {{
                "f:spec": {{
                  "f:settings": {{
                    ".": {{}},
                    "f:enableMultiTenancy": {{}}
                  }}
                }}
              }},
              "manager": "kubectl-edit",
              "operation": "Update",
              "time": "2025-07-30T00:58:26Z"
            }}
          ],
          "name": "user-authz",
          "resourceVersion": "663947",
          "uid": "71324cad-b74b-45ce-b122-1040558471ee"
        }},
        "spec": {{
          "enabled": true,
          "settings": {{
            {'' if module_enable_multitenancy_field is None else ('"enableMultiTenancy": true' if module_enable_multitenancy_field else '"enableMultiTenancy": false')}
          }},
          "version": 1
        }},
        "status": {{
          "message": "",
          "version": "1"
        }}
      }},
      "oldObject": {{
        "apiVersion": "deckhouse.io/v1alpha1",
        "kind": "ModuleConfig",
        "metadata": {{
          "creationTimestamp": "2025-07-29T02:01:51Z",
          "finalizers": [
            "modules.deckhouse.io/module-registered"
          ],
          "generation": 19,
          "managedFields": [
            {{
              "apiVersion": "deckhouse.io/v1alpha1",
              "fieldsType": "FieldsV1",
              "fieldsV1": {{
                "f:spec": {{
                  ".": {{}},
                  "f:enabled": {{}},
                  "f:version": {{}}
                }}
              }},
              "manager": "dhctl",
              "operation": "Update",
              "time": "2025-07-29T02:01:51Z"
            }},
            {{
              "apiVersion": "deckhouse.io/v1alpha1",
              "fieldsType": "FieldsV1",
              "fieldsV1": {{
                "f:metadata": {{
                  "f:finalizers": {{
                    ".": {{}}
                  }}
                }}
              }},
              "manager": "deckhouse-controller",
              "operation": "Update",
              "time": "2025-07-29T02:02:28Z"
            }},
            {{
              "apiVersion": "deckhouse.io/v1alpha1",
              "fieldsType": "FieldsV1",
              "fieldsV1": {{
                "f:status": {{
                  ".": {{}},
                  "f:message": {{}},
                  "f:version": {{}}
                }}
              }},
              "manager": "deckhouse-controller",
              "operation": "Update",
              "subresource": "status",
              "time": "2025-07-29T02:02:28Z"
            }},
            {{
              "apiVersion": "deckhouse.io/v1alpha1",
              "fieldsType": "FieldsV1",
              "fieldsV1": {{
                "f:spec": {{
                  "f:settings": {{
                    ".": {{}},
                    "f:enableMultiTenancy": {{}}
                  }}
                }}
              }},
              "manager": "kubectl-edit",
              "operation": "Update",
              "time": "2025-07-29T23:27:00Z"
            }}
          ],
          "name": "user-authz",
          "resourceVersion": "663947",
          "uid": "71324cad-b74b-45ce-b122-1040558471ee"
        }},
        "spec": {{
          "enabled": true,
          "settings": {{}},
          "version": 1
        }},
        "status": {{
          "message": "",
          "version": "1"
        }}
      }},
      "dryRun": false,
      "options": {{
        "kind": "UpdateOptions",
        "apiVersion": "meta.k8s.io/v1",
        "fieldManager": "kubectl-edit",
        "fieldValidation": "Strict"
      }}
    }}
  }},
  "snapshots": {{
    "d8-user-authz-cars": [
      {cars_snapshot if cars else ""}
    ]
  }},
  "type": "Validating"
}}
"""
