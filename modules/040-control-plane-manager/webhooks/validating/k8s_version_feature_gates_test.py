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
    validate,
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
    default_version: str = "1.30.0",
    mc_kubernetes_version: str = None,
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
                    "kubeadm:cluster-admins",
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
    encoded_default_version = base64.b64encode(default_version.encode('utf-8')).decode('utf-8')
    ctx.review.request.oldObject.data['deckhouseDefaultKubernetesVersion'] = encoded_default_version
    ctx.review.request.object.data['deckhouseDefaultKubernetesVersion'] = encoded_default_version
    
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
    
    if enabled_feature_gates or mc_kubernetes_version:
        settings = {}
        if enabled_feature_gates:
            settings["enabledFeatureGates"] = enabled_feature_gates
        # ModuleConfig owns the version now, so what it says decides whether a ClusterConfiguration
        # edit can move the effective version at all.
        if mc_kubernetes_version:
            settings["kubernetesVersion"] = mc_kubernetes_version
        module_config_snapshot = [DotMap({
            "object": {
                "apiVersion": "deckhouse.io/v1alpha1",
                "kind": "ModuleConfig",
                "metadata": {
                    "name": "control-plane-manager"
                },
                "spec": {
                    "settings": settings
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
            "The following feature gates are deprecated in this version or earlier: 'DynamicResourceAllocation'\n"
            "You can remove them from the enabledFeatureGates in the control-plane-manager ModuleConfig."
        )
        tests.assert_validation_deny(self, out, error_msg)
    
    def test_validate_version_change_with_non_deprecated_feature_gate_should_allow(self):
        ctx = _prepare_validation_binding_context('1.29.0', '1.30.0', ['CPUManager'])
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)
    
    def test_validate_upgrade_to_version_higher_than_deprecated_should_reject(self):
        ctx = _prepare_validation_binding_context('1.30.0', '1.33.0', ['New123'])
        out = hook.testrun(main, [ctx])
        error_msg = (
            "Cannot change Kubernetes version to 1.33.0:\n"
            "The following feature gates are deprecated in this version or earlier: 'New123'\n"
            "You can remove them from the enabledFeatureGates in the control-plane-manager ModuleConfig."
        )
        tests.assert_validation_deny(self, out, error_msg)
    
    def test_validate_upgrade_to_exact_deprecated_version_should_reject(self):
        ctx = _prepare_validation_binding_context('1.30.0', '1.32.0', ['New123'])
        out = hook.testrun(main, [ctx])
        error_msg = (
            "Cannot change Kubernetes version to 1.32.0:\n"
            "The following feature gates are deprecated in this version or earlier: 'New123'\n"
            "You can remove them from the enabledFeatureGates in the control-plane-manager ModuleConfig."
        )
        tests.assert_validation_deny(self, out, error_msg)
    
    def test_validate_upgrade_to_version_before_deprecated_should_allow(self):
        ctx = _prepare_validation_binding_context('1.29.0', '1.30.0', ['DynamicResourceAllocation'])
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_removing_cc_version_uses_default_and_rejects_deprecated_feature_gate(self):
        ctx = _prepare_validation_binding_context(
            '1.30.0', None, ['New123'], default_version='1.32.0',
        )
        out = hook.testrun(main, [ctx])
        error_msg = (
            "Cannot change Kubernetes version to 1.32.0:\n"
            "The following feature gates are deprecated in this version or earlier: 'New123'\n"
            "You can remove them from the enabledFeatureGates in the control-plane-manager ModuleConfig."
        )
        tests.assert_validation_deny(self, out, error_msg)

    def test_removing_cc_version_is_allowed_when_module_config_pins_the_version(self):
        """The documented migration step must not be blocked.

        Operators are told (by D8UnsetKubernetesVersionInModuleConfig) to move the pin into
        ModuleConfig and drop the deprecated ClusterConfiguration field. Once ModuleConfig pins a
        version, resolve_effective_version never consults ClusterConfiguration.kubernetesVersion, so
        this edit cannot change the effective version and must not be denied — not even when a
        deprecated feature gate is enabled.
        """
        ctx = _prepare_validation_binding_context(
            '1.30.0', None, ['New123'], default_version='1.32.0',
            mc_kubernetes_version='1.32',
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_removing_cc_version_is_allowed_when_module_config_tracks_default(self):
        """Default/Automatic in ModuleConfig also takes ClusterConfiguration out of the picture."""
        ctx = _prepare_validation_binding_context(
            '1.30.0', None, ['New123'], default_version='1.32.0',
            mc_kubernetes_version='Default',
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_validate_missing_kind_should_allow_not_raise(self):
        # DotMap returns an empty DotMap for missing keys; .kind.kind.lower() would
        # AttributeError and lock ModuleConfig edits under failurePolicy:Fail.
        ctx = DotMap({"review": {"request": {}}})
        self.assertIsNone(validate(ctx))


def _prepare_mc_validation_binding_context(
    mc_k8s_version,
    enabled_feature_gates,
    cc_k8s_version: str = None,
    cc_default_version: str = "1.30.0",
    cc_snapshot_present: bool = None,
) -> DotMap:
    binding_context_json = """
{
    "binding": "cpm-k8s-version-feature-gates-mc.deckhouse.io",
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
                    "kubeadm:cluster-admins",
                    "system:authenticated"
                ]
            },
            "object": {
                "apiVersion": "deckhouse.io/v1",
                "kind": "ModuleConfig",
                "metadata": {
                    "name": "control-plane-manager"
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

    if enabled_feature_gates is not None:
        ctx.review.request.object.spec.settings.enabledFeatureGates = enabled_feature_gates

    if mc_k8s_version is not None:
        ctx.review.request.object.spec.settings.kubernetesVersion = mc_k8s_version

    if cc_snapshot_present is None:
        cc_snapshot_present = cc_k8s_version is not None

    if cc_snapshot_present:
        cluster_config = {}
        if cc_k8s_version is not None:
            cluster_config['kubernetesVersion'] = cc_k8s_version
        cluster_config_yaml = yaml.dump(cluster_config)
        encoded_config = base64.b64encode(cluster_config_yaml.encode('utf-8')).decode('utf-8')
        encoded_default_version = base64.b64encode(cc_default_version.encode('utf-8')).decode('utf-8')

        secret_snapshot = [DotMap({
            "object": {
                "data": {
                    "cluster-configuration.yaml": encoded_config,
                    "deckhouseDefaultKubernetesVersion": encoded_default_version
                }
            }
        })]
        ctx.snapshots[CLUSTER_CONFIG_SNAPSHOT_NAME] = secret_snapshot
    else:
        ctx.snapshots[CLUSTER_CONFIG_SNAPSHOT_NAME] = []

    return ctx


def _with_old_object(ctx: DotMap, old_k8s_version, old_enabled_feature_gates) -> DotMap:
    """Attach an oldObject, turning the request into a real UPDATE.

    The cases above leave oldObject null, which stands for CREATE — there the webhook has nothing
    to compare against and always runs the full check."""
    old_settings = {}
    if old_k8s_version is not None:
        old_settings['kubernetesVersion'] = old_k8s_version
    if old_enabled_feature_gates is not None:
        old_settings['enabledFeatureGates'] = old_enabled_feature_gates

    ctx.review.request.oldObject = DotMap({
        "apiVersion": "deckhouse.io/v1",
        "kind": "ModuleConfig",
        "metadata": {
            "name": "control-plane-manager"
        },
        "spec": {
            "settings": old_settings
        }
    })

    return ctx


class TestK8sVersionFeatureGatesModuleConfigTrigger(unittest.TestCase):
    """Covers the ModuleConfig-triggered binding: kubernetesVersion changed via
    `kubectl edit mc control-plane-manager` never touches the d8-cluster-configuration
    secret, so the secret-triggered binding above never fires for it."""

    def test_mc_version_with_deprecated_feature_gate_should_reject(self):
        ctx = _prepare_mc_validation_binding_context('1.33.0', ['DynamicResourceAllocation'])
        out = hook.testrun(main, [ctx])
        error_msg = (
            "Cannot change Kubernetes version to 1.33.0:\n"
            "The following feature gates are deprecated in this version or earlier: 'DynamicResourceAllocation'\n"
            "You can remove them from the enabledFeatureGates in the control-plane-manager ModuleConfig."
        )
        tests.assert_validation_deny(self, out, error_msg)

    def test_mc_version_with_non_deprecated_feature_gate_should_allow(self):
        ctx = _prepare_mc_validation_binding_context('1.30.0', ['CPUManager'])
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_mc_without_kubernetes_version_falls_back_to_cluster_configuration(self):
        ctx = _prepare_mc_validation_binding_context(None, ['New123'], cc_k8s_version='1.32.0')
        out = hook.testrun(main, [ctx])
        error_msg = (
            "Cannot change Kubernetes version to 1.32.0:\n"
            "The following feature gates are deprecated in this version or earlier: 'New123'\n"
            "You can remove them from the enabledFeatureGates in the control-plane-manager ModuleConfig."
        )
        tests.assert_validation_deny(self, out, error_msg)

    def test_mc_automatic_version_uses_deckhouse_default(self):
        ctx = _prepare_mc_validation_binding_context(
            'Automatic', ['CPUManager'], cc_k8s_version='Automatic', cc_default_version='1.30.0',
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_mc_automatic_ignores_pinned_cluster_configuration(self):
        # Presence of the ModuleConfig setting decides: an explicit Automatic resolves to the
        # Deckhouse default, so the deprecated ClusterConfiguration pin must not be consulted.
        # 'New123' is deprecated in 1.32 (the default here) but not in 1.31 (the CC pin), so the
        # deny below only happens if the default won.
        ctx = _prepare_mc_validation_binding_context(
            'Automatic', ['New123'], cc_k8s_version='1.31.0', cc_default_version='1.32.0',
        )
        out = hook.testrun(main, [ctx])
        error_msg = (
            "Cannot change Kubernetes version to 1.32.0:\n"
            "The following feature gates are deprecated in this version or earlier: 'New123'\n"
            "You can remove them from the enabledFeatureGates in the control-plane-manager ModuleConfig."
        )
        tests.assert_validation_deny(self, out, error_msg)

    def test_mc_automatic_with_cc_version_absent_uses_default(self):
        ctx = _prepare_mc_validation_binding_context(
            'Automatic',
            ['New123'],
            cc_default_version='1.32.0',
            cc_snapshot_present=True,
        )
        out = hook.testrun(main, [ctx])
        error_msg = (
            "Cannot change Kubernetes version to 1.32.0:\n"
            "The following feature gates are deprecated in this version or earlier: 'New123'\n"
            "You can remove them from the enabledFeatureGates in the control-plane-manager ModuleConfig."
        )
        tests.assert_validation_deny(self, out, error_msg)

    def test_mc_without_kubernetes_version_and_without_cluster_configuration_should_allow(self):
        ctx = _prepare_mc_validation_binding_context(None, ['New123'])
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_mc_without_feature_gates_should_allow(self):
        ctx = _prepare_mc_validation_binding_context('1.33.0', None)
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_mc_unrelated_change_should_allow_even_with_deprecated_feature_gate(self):
        """A ModuleConfig already carrying a deprecated feature gate must stay editable.

        Without an old/new comparison the check re-runs on every edit, so changing an unrelated
        setting would be rejected with a message about a kubernetesVersion nobody touched — and a
        Deckhouse upgrade extending the deprecation table would put clusters in that state on its
        own, with no user action at all."""
        ctx = _prepare_mc_validation_binding_context('1.33.0', ['DynamicResourceAllocation'])
        ctx.review.request.object.spec.settings.rootKubeconfigSymlink = False
        ctx = _with_old_object(ctx, '1.33.0', ['DynamicResourceAllocation'])
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_mc_adding_deprecated_feature_gate_should_still_reject(self):
        ctx = _prepare_mc_validation_binding_context('1.33.0', ['DynamicResourceAllocation'])
        ctx = _with_old_object(ctx, '1.33.0', [])
        out = hook.testrun(main, [ctx])
        error_msg = (
            "Cannot change Kubernetes version to 1.33.0:\n"
            "The following feature gates are deprecated in this version or earlier: 'DynamicResourceAllocation'\n"
            "You can remove them from the enabledFeatureGates in the control-plane-manager ModuleConfig."
        )
        tests.assert_validation_deny(self, out, error_msg)

    def test_mc_changing_version_should_still_reject(self):
        ctx = _prepare_mc_validation_binding_context('1.33.0', ['DynamicResourceAllocation'])
        ctx = _with_old_object(ctx, '1.30.0', ['DynamicResourceAllocation'])
        out = hook.testrun(main, [ctx])
        error_msg = (
            "Cannot change Kubernetes version to 1.33.0:\n"
            "The following feature gates are deprecated in this version or earlier: 'DynamicResourceAllocation'\n"
            "You can remove them from the enabledFeatureGates in the control-plane-manager ModuleConfig."
        )
        tests.assert_validation_deny(self, out, error_msg)


if __name__ == '__main__':
    unittest.main()
