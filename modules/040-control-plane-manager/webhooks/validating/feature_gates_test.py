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
from unittest.mock import patch, mock_open

from feature_gates import main, validate
from deckhouse import hook, tests
from dotmap import DotMap


def _prepare_validation_binding_context(k8s_version: str, enabled_feature_gates: list) -> DotMap:
    binding_context_json = """
{
    "binding": "cpm-moduleconfig-feature-gates.deckhouse.io",
    "review": {
        "request": {
            "uid": "8af60184-b30b-4b90-a33e-0c190f10e96d",
            "kind": {
                "group": "deckhouse.io",
                "version": "v1",
                "kind": "ModuleConfig"
            },
            "resource": {
                "group": "deckhouse.io",
                "version": "v1",
                "resource": "moduleconfigs"
            },
            "requestKind": {
                "group": "deckhouse.io",
                "version": "v1",
                "kind": "ModuleConfig"
            },
            "requestResource": {
                "group": "deckhouse.io",
                "version": "v1",
                "resource": "moduleconfigs"
            },
            "name": "control-plane-manager",
            "operation": "UPDATE",
            "userInfo": {
                "username": "kubernetes-admin",
                "groups": [
                    "system:masters",
                    "system:authenticated"
                ]
            },
            "object": {
                "apiVersion": "deckhouse.io/v1",
                "kind": "ModuleConfig",
                "metadata": {
                    "creationTimestamp": "2023-07-17T13:40:39Z",
                    "generation": 3,
                    "name": "control-plane-manager",
                    "resourceVersion": "1184522270",
                    "uid": "7820c68b-6423-49f0-b97f-b7e314e23c0b"
                },
                "spec": {
                    "settings": {}
                }
            },
            "oldObject": null,
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
    ctx.review.request.object.spec.settings.enabledFeatureGates = enabled_feature_gates
    return ctx


class TestFeatureGatesValidationWebhook(unittest.TestCase):
    
    @patch('feature_gates.os.path.exists')
    @patch('feature_gates.os.path.getsize')
    @patch('feature_gates.open', new_callable=mock_open, read_data='1.29.0\n')
    def test_validate_with_valid_feature_gate_should_allow(self, mock_file, mock_getsize, mock_exists):
        mock_exists.return_value = True
        mock_getsize.return_value = 10
        ctx = _prepare_validation_binding_context('1.29.0', ['CPUManager'])
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)
    
    @patch('feature_gates.os.path.exists')
    @patch('feature_gates.os.path.getsize')
    @patch('feature_gates.open', new_callable=mock_open, read_data='1.33.0\n')
    def test_validate_with_forbidden_feature_gate_should_warn(self, mock_file, mock_getsize, mock_exists):
        mock_exists.return_value = True
        mock_getsize.return_value = 10
        ctx = _prepare_validation_binding_context('1.33.0', ['SomeProblematicFeature'])
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, "'SomeProblematicFeature' is forbidden for Kubernetes version 1.33 and will not be applied")
    
    @patch('feature_gates.os.path.exists')
    @patch('feature_gates.os.path.getsize')
    @patch('feature_gates.open', new_callable=mock_open, read_data='1.29.0\n')
    def test_validate_with_multiple_feature_gates(self, mock_file, mock_getsize, mock_exists):
        mock_exists.return_value = True
        mock_getsize.return_value = 10
        ctx = _prepare_validation_binding_context('1.29.0', ['CPUManager', 'MemoryManager', 'UnknownGate'])
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, "'UnknownGate' is unknown or enabled by default FeatureGate for Kubernetes version 1.29 and will not be applied")
    
    
    @patch('feature_gates.os.path.exists')
    @patch('feature_gates.os.path.getsize')
    @patch('feature_gates.open', new_callable=mock_open, read_data='1.30.0\n')
    def test_validate_with_apiserver_feature_gate(self, mock_file, mock_getsize, mock_exists):
        mock_exists.return_value = True
        mock_getsize.return_value = 10
        ctx = _prepare_validation_binding_context('1.30.0', ['APIServerIdentity'])
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)
    
    @patch('feature_gates.os.path.exists')
    @patch('feature_gates.os.path.getsize')
    @patch('feature_gates.open', new_callable=mock_open, read_data='1.30.0\n')
    def test_validate_with_kubecontroller_manager_feature_gate(self, mock_file, mock_getsize, mock_exists):
        mock_exists.return_value = True
        mock_getsize.return_value = 10
        ctx = _prepare_validation_binding_context('1.30.0', ['CronJobsScheduledAnnotation'])
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)
    
    @patch('feature_gates.os.path.exists')
    @patch('feature_gates.os.path.getsize')
    @patch('feature_gates.open', new_callable=mock_open, read_data='1.30.0\n')
    def test_validate_with_scheduler_feature_gate(self, mock_file, mock_getsize, mock_exists):
        mock_exists.return_value = True
        mock_getsize.return_value = 10
        ctx = _prepare_validation_binding_context('1.30.0', ['SchedulerQueueingHints'])
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)
    
    @patch('feature_gates.os.path.exists')
    @patch('feature_gates.os.path.getsize')
    @patch('feature_gates.open', new_callable=mock_open, read_data='1.29.0\n')
    def test_validate_with_missing_feature_gates_should_allow(self, mock_file, mock_getsize, mock_exists):
        mock_exists.return_value = True
        mock_getsize.return_value = 10
        ctx = _prepare_validation_binding_context('1.29.0', None)
        if hasattr(ctx.review.request.object.spec.settings, 'enabledFeatureGates'):
            del ctx.review.request.object.spec.settings.enabledFeatureGates
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)
    
    @patch('feature_gates.os.path.exists')
    @patch('feature_gates.os.path.getsize')
    @patch('feature_gates.open', new_callable=mock_open, read_data='1.29.0\n')
    def test_validate_with_none_feature_gates_should_allow(self, mock_file, mock_getsize, mock_exists):
        mock_exists.return_value = True
        mock_getsize.return_value = 10
        ctx = _prepare_validation_binding_context('1.29.0', None)
        ctx.review.request.object.spec.settings.enabledFeatureGates = None
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)
    
    @patch('feature_gates.os.path.exists')
    @patch('feature_gates.os.path.getsize')
    @patch('feature_gates.open', new_callable=mock_open, read_data='1.33.0\n')
    def test_validate_with_forbidden_and_deprecated_feature_gates(self, mock_file, mock_getsize, mock_exists):
        mock_exists.return_value = True
        mock_getsize.return_value = 10
        ctx = _prepare_validation_binding_context('1.33.0', ['SomeProblematicFeature', 'DynamicResourceAllocation'])
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

if __name__ == '__main__':
    unittest.main()

