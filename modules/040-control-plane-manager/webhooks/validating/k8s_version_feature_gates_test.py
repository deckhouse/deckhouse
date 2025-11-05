#!/usr/bin/python3

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
import json
import base64
import yaml

import feature_gates_generated
from k8s_version_feature_gates import (
    main,
    get_enabled_feature_gates,
    normalize_version,
    CLUSTER_CONFIG_SNAPSHOT_NAME,
    MODULE_CONFIG_SNAPSHOT_NAME,
)
from deckhouse import hook, tests
from dotmap import DotMap


TEST_FEATURE_GATES_MAP = {
    "1.29": {
        "kubelet": ["CPUManager", "MemoryManager"],
        "apiserver": ["APIServerIdentity", "StorageVersionAPI"],
        "kubeControllerManager": ["CronJobsScheduledAnnotation"],
        "kubeScheduler": ["SchedulerQueueingHints"],
    },
    "1.30": {
        "kubelet": ["CPUManager", "MemoryManager"],
        "apiserver": ["APIServerIdentity", "StorageVersionAPI"],
        "kubeControllerManager": ["CronJobsScheduledAnnotation"],
        "kubeScheduler": ["SchedulerQueueingHints"],
    },
    "1.32": {
        "deprecated": ["New123"],
        "forbidden": ["SomeProblematicFeature"],
        "kubelet": ["CPUManager", "MemoryManager"],
        "apiserver": ["APIServerIdentity", "StorageVersionAPI"],
        "kubeControllerManager": ["CronJobsScheduledAnnotation"],
        "kubeScheduler": ["SchedulerQueueingHints"],
    },
    "1.33": {
        "deprecated": ["DynamicResourceAllocation"],
        "forbidden": ["SomeProblematicFeature"],
        "kubelet": ["CPUManager", "MemoryManager"],
        "apiserver": ["APIServerIdentity", "StorageVersionAPI"],
        "kubeControllerManager": ["CronJobsScheduledAnnotation"],
        "kubeScheduler": ["SchedulerQueueingHints"],
    },
}

feature_gates_generated.versions = TEST_FEATURE_GATES_MAP


def _prepare_validation_binding_context(
    old_k8s_version: str,
    new_k8s_version: str,
    enabled_feature_gates: list,
) -> DotMap:
    binding_context_json = """
{
    "binding": "cpm-k8s-version-feature-gates.deckhouse.io",
    "review": {
        "request": {
            "uid": "8af60184-b30b-4b90-a33e-0c190f10e96d",
            "kind": {
                "group": "",
                "version": "v1",
                "kind": "Secret"
            },
            "resource": {
                "group": "",
                "version": "v1",
                "resource": "secrets"
            },
            "requestKind": {
                "group": "",
                "version": "v1",
                "kind": "Secret"
            },
            "requestResource": {
                "group": "",
                "version": "v1",
                "resource": "secrets"
            },
            "name": "d8-cluster-configuration",
            "namespace": "kube-system",
            "operation": "UPDATE",
            "userInfo": {
                "username": "kubernetes-admin",
                "groups": [
                    "system:masters",
                    "system:authenticated"
                ]
            },
            "object": {
                "apiVersion": "v1",
                "kind": "Secret",
                "metadata": {
                    "name": "d8-cluster-configuration",
                    "namespace": "kube-system",
                    "creationTimestamp": "2023-07-17T13:40:39Z",
                    "labels": {
                        "name": "d8-cluster-configuration"
                    }
                },
                "data": {}
            },
            "oldObject": {
                "apiVersion": "v1",
                "kind": "Secret",
                "metadata": {
                    "name": "d8-cluster-configuration",
                    "namespace": "kube-system",
                    "creationTimestamp": "2023-07-17T13:40:39Z",
                    "labels": {
                        "name": "d8-cluster-configuration"
                    }
                },
                "data": {}
            },
            "dryRun": false,
            "options": {
                "kind": "UpdateOptions",
                "apiVersion": "meta.k8s.io/v1",
                "fieldManager": "kubectl-edit",
                "fieldValidation": "Strict"
            }
        }
    },
    "snapshots": {},
    "type": "Validating"
}
"""
    ctx_dict = json.loads(binding_context_json)
    ctx = DotMap(ctx_dict)
    
    if old_k8s_version:
        old_cluster_config = {'kubernetesVersion': old_k8s_version}
        old_cluster_config_yaml = yaml.dump(old_cluster_config)
        encoded_old_config = base64.b64encode(old_cluster_config_yaml.encode('utf-8')).decode('utf-8')
        ctx.review.request.oldObject.data['cluster-configuration.yaml'] = encoded_old_config
    
    if new_k8s_version:
        new_cluster_config = {'kubernetesVersion': new_k8s_version}
        new_cluster_config_yaml = yaml.dump(new_cluster_config)
        encoded_new_config = base64.b64encode(new_cluster_config_yaml.encode('utf-8')).decode('utf-8')
        ctx.review.request.object.data['cluster-configuration.yaml'] = encoded_new_config
    
    if enabled_feature_gates:
        module_config_snapshot = [DotMap({
            "object": {
                "apiVersion": "deckhouse.io/v1",
                "kind": "ModuleConfig",
                "metadata": {
                    "name": "control-plane-manager"
                },
                "spec": {
                    "settings": {
                        "enabledFeatureGates": enabled_feature_gates
                    }
                }
            }
        })]
        ctx.snapshots[MODULE_CONFIG_SNAPSHOT_NAME] = module_config_snapshot
    else:
        ctx.snapshots[MODULE_CONFIG_SNAPSHOT_NAME] = []
    
    return ctx


class TestK8sVersionFeatureGatesValidationWebhook(unittest.TestCase):
    
    def test_validate_same_version_should_allow(self):
        ctx = _prepare_validation_binding_context('1.29.0', '1.29.0', ['CPUManager'])
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)
    
    def test_validate_version_change_without_feature_gates_should_allow(self):
        ctx = _prepare_validation_binding_context('1.29.0', '1.30.0', [])
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)
    
    def test_validate_version_change_without_module_config_should_allow(self):
        ctx = _prepare_validation_binding_context('1.29.0', '1.30.0', None)
        if MODULE_CONFIG_SNAPSHOT_NAME in ctx.snapshots:
            del ctx.snapshots[MODULE_CONFIG_SNAPSHOT_NAME]
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)
    
    def test_validate_version_change_with_deprecated_feature_gate_should_reject(self):
        ctx = _prepare_validation_binding_context('1.32.0', '1.33.0', ['DynamicResourceAllocation'])
        out = hook.testrun(main, [ctx])
        error_msg = (
            "Cannot change Kubernetes version to 1.33.0:\n"
            "The following feature gates are deprecated in this version: 'DynamicResourceAllocation'\n"
            "You can remove them from the enabledFeatureGates in the control-plane-manager ModuleConfig."
        )
        tests.assert_validation_deny(self, out, error_msg)
    
    def test_validate_version_change_with_non_deprecated_feature_gate_should_allow(self):
        ctx = _prepare_validation_binding_context('1.29.0', '1.30.0', ['CPUManager'])
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

if __name__ == '__main__':
    unittest.main()

