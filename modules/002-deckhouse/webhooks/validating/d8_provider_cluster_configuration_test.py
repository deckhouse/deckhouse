#!/usr/bin/env python3

# Copyright 2024 Flant JSC
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

import base64
import unittest
import yaml
from d8_provider_cluster_configuration import (
    main,
    REGISTRY_MODES_WITHOUT_REGISTRY_DATA_DEVICE_SUPPORT,
)
from dotmap import DotMap
from deckhouse import hook, tests


def _prepare_update_binding_context(
    old_data, new_data: Optional[dict], cluster_cfg_data: Optional[bytes]
) -> DotMap:
    ctx = DotMap(
        {
            "binding": "d8-provider-cluster-configuration-secret.deckhouse.io",
            "review": {
                "request": {
                    "uid": "8af60184-b30b-4b90-a33e-0c190f10e96d",
                    "kind": {
                        "group": "",
                        "version": "v1",
                        "kind": "Secret",
                    },
                    "resource": {
                        "group": "",
                        "version": "v1",
                        "resource": "secrets",
                    },
                    "requestKind": {
                        "group": "",
                        "version": "v1",
                        "kind": "Secret",
                    },
                    "requestResource": {
                        "group": "",
                        "version": "v1",
                        "resource": "secrets",
                    },
                    "name": "d8-cluster-configuration",
                    "operation": "UPDATE",
                    "userInfo": {
                        "username": "kubernetes-admin",
                        "groups": ["system:masters", "system:authenticated"],
                    },
                    "object": {
                        "apiVersion": "v1",
                        "kind": "Secret",
                        "metadata": {
                            "creationTimestamp": "2023-07-17T13:40:39Z",
                            "name": "d8-cluster-configuration",
                            "namespace": "kube-system",
                            "resourceVersion": "1184522270",
                            "uid": "7820c68b-6423-49f0-b97f-b7e314e23c0b",
                        },
                        "type": "Opaque",
                        "data": {},
                    },
                    "oldObject": {
                        "apiVersion": "v1",
                        "kind": "Secret",
                        "metadata": {
                            "creationTimestamp": "2023-07-17T13:40:39Z",
                            "name": "d8-cluster-configuration",
                            "namespace": "kube-system",
                            "resourceVersion": "1184522270",
                            "uid": "7820c68b-6423-49f0-b97f-b7e314e23c0b",
                        },
                        "type": "Opaque",
                        "data": {},
                    },
                    "dryRun": False,
                    "options": {
                        "kind": "UpdateOptions",
                        "apiVersion": "meta.k8s.io/v1",
                        "fieldManager": "kubectl-edit",
                        "fieldValidation": "Strict",
                    },
                }
            },
            "snapshots": {
                "cluster_cfg": [],
            },
            "type": "Validating",
        }
    )

    if new_data is not None:
        ctx.review.request.object.data = new_data
    if old_data is not None:
        ctx.review.request.oldObject.data = old_data
    if cluster_cfg_data is not None:
        ctx.snapshots.cluster_cfg = [
            dict(
                {
                    "filterResult": cluster_cfg_data,
                }
            )
        ]
    return ctx


def _prepare_create_binding_context(
    new_data: Optional[dict], cluster_cfg_data: Optional[bytes]
) -> DotMap:
    ctx = DotMap(
        {
            "binding": "d8-provider-cluster-configuration-secret.deckhouse.io",
            "review": {
                "request": {
                    "uid": "8af60184-b30b-4b90-a33e-0c190f10e96d",
                    "kind": {
                        "group": "",
                        "version": "v1",
                        "kind": "Secret",
                    },
                    "resource": {
                        "group": "",
                        "version": "v1",
                        "resource": "secrets",
                    },
                    "requestKind": {
                        "group": "",
                        "version": "v1",
                        "kind": "Secret",
                    },
                    "requestResource": {
                        "group": "",
                        "version": "v1",
                        "resource": "secrets",
                    },
                    "name": "d8-cluster-configuration",
                    "operation": "CREATE",
                    "userInfo": {
                        "username": "kubernetes-admin",
                        "groups": ["system:masters", "system:authenticated"],
                    },
                    "object": {
                        "apiVersion": "v1",
                        "kind": "Secret",
                        "metadata": {
                            "creationTimestamp": "2023-07-17T13:40:39Z",
                            "name": "d8-cluster-configuration",
                            "namespace": "kube-system",
                            "resourceVersion": "1184522270",
                            "uid": "7820c68b-6423-49f0-b97f-b7e314e23c0b",
                        },
                        "type": "Opaque",
                        "data": {},
                    },
                    "dryRun": False,
                    "options": {
                        "kind": "UpdateOptions",
                        "apiVersion": "meta.k8s.io/v1",
                        "fieldManager": "kubectl-edit",
                        "fieldValidation": "Strict",
                    },
                }
            },
            "snapshots": {
                "cluster_cfg": [],
            },
            "type": "Validating",
        }
    )

    if new_data is not None:
        ctx.review.request.object.data = new_data
    if cluster_cfg_data is not None:
        ctx.snapshots.cluster_cfg = [
            dict(
                {
                    "filterResult": cluster_cfg_data,
                }
            )
        ]
    return ctx


def _prepare_delete_binding_context(
    old_data: Optional[dict], cluster_cfg_data: Optional[bytes]
) -> DotMap:
    ctx = DotMap(
        {
            "binding": "d8-provider-cluster-configuration-secret.deckhouse.io",
            "review": {
                "request": {
                    "uid": "8af60184-b30b-4b90-a33e-0c190f10e96d",
                    "kind": {
                        "group": "",
                        "version": "v1",
                        "kind": "Secret",
                    },
                    "resource": {
                        "group": "",
                        "version": "v1",
                        "resource": "secrets",
                    },
                    "requestKind": {
                        "group": "",
                        "version": "v1",
                        "kind": "Secret",
                    },
                    "requestResource": {
                        "group": "",
                        "version": "v1",
                        "resource": "secrets",
                    },
                    "name": "d8-cluster-configuration",
                    "operation": "DELETE",
                    "userInfo": {
                        "username": "kubernetes-admin",
                        "groups": ["system:masters", "system:authenticated"],
                    },
                    "oldObject": {
                        "apiVersion": "v1",
                        "kind": "Secret",
                        "metadata": {
                            "creationTimestamp": "2023-07-17T13:40:39Z",
                            "name": "d8-cluster-configuration",
                            "namespace": "kube-system",
                            "resourceVersion": "1184522270",
                            "uid": "7820c68b-6423-49f0-b97f-b7e314e23c0b",
                        },
                        "type": "Opaque",
                        "data": {},
                    },
                    "dryRun": False,
                    "options": {
                        "kind": "UpdateOptions",
                        "apiVersion": "meta.k8s.io/v1",
                        "fieldManager": "kubectl-edit",
                        "fieldValidation": "Strict",
                    },
                }
            },
            "snapshots": {
                "cluster_cfg": [],
            },
            "type": "Validating",
        }
    )

    if old_data is not None:
        ctx.review.request.oldObject.data = old_data
    if cluster_cfg_data is not None:
        ctx.snapshots.cluster_cfg = [
            dict(
                {
                    "filterResult": cluster_cfg_data,
                }
            )
        ]
    return ctx


def _prepare_cloud_provider_secondary_devices_cfg_data(
    registry_data_device_enable: bool,
) -> bytes:
    data = dict(
        {
            "RegistryDataDeviceEnable": registry_data_device_enable,
        }
    )
    yaml_data = yaml.dump(data, default_flow_style=False).encode("utf-8")
    return base64.standard_b64encode(yaml_data)


def _prepare_provider_cluster_configuration_with_custom_data(
    data: bytes,
) -> dict:
    return dict(
        {
            "cloud-provider-secondary-devices-configuration.yaml": data,
        }
    )


def _prepare_provider_cluster_configuration(
    registry_data_device_enable: bool,
) -> dict:
    return _prepare_provider_cluster_configuration_with_custom_data(
        _prepare_cloud_provider_secondary_devices_cfg_data(
            registry_data_device_enable=registry_data_device_enable
        )
    )


def _prepare_cluster_cfg_with_static() -> bytes:
    data = dict({"apiVersion": "deckhouse.io/v1", "static": {}})
    yaml_data = yaml.dump(data, default_flow_style=False).encode("utf-8")
    return base64.standard_b64encode(yaml_data)


def _prepare_cluster_cfg_with_supported_cloud() -> bytes:
    data = dict(
        {
            "apiVersion": "deckhouse.io/v1",
            "cloud": {
                "provider": "YaNdEx",
            },
        }
    )
    yaml_data = yaml.dump(data, default_flow_style=False).encode("utf-8")
    return base64.standard_b64encode(yaml_data)


def _prepare_cluster_cfg_with_unsupported_cloud() -> bytes:
    data = dict(
        {
            "apiVersion": "deckhouse.io/v1",
            "cloud": {
                "provider": "NonYandex",
            },
        }
    )
    yaml_data = yaml.dump(data, default_flow_style=False).encode("utf-8")
    return base64.standard_b64encode(yaml_data)


class TestProviderSecondaryDevicesConfigurationCreate(unittest.TestCase):

    def test_should_create_with_enabled_registry_data_device_with_supported_cloud(
        self,
    ):
        ctx = _prepare_create_binding_context(
            new_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=True
            ),
            cluster_cfg_data=_prepare_cluster_cfg_with_supported_cloud(),
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_should_create_with_enabled_registry_data_device_with_static_cluster(
        self,
    ):
        ctx = _prepare_create_binding_context(
            new_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=True
            ),
            cluster_cfg_data=_prepare_cluster_cfg_with_static(),
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_should_create_with_empty_configs(
        self,
    ):
        ctx = _prepare_create_binding_context(
            new_data=None,
            cluster_cfg_data=None,
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_should_create_with_empty_fields(
        self,
    ):
        ctx = _prepare_create_binding_context(
            new_data=_prepare_provider_cluster_configuration_with_custom_data(""),
            cluster_cfg_data="",
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_should_not_create_with_enabled_registry_data_device_with_empty_cluster_cfg(
        self,
    ):
        ctx = _prepare_create_binding_context(
            new_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=True
            ),
            cluster_cfg_data="",
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(
            self, out, "Cannot get provider cluster configuration"
        )

    def test_should_not_create_with_enabled_registry_data_device_with_unsupported_cloud(
        self,
    ):
        ctx = _prepare_create_binding_context(
            new_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=True
            ),
            cluster_cfg_data=_prepare_cluster_cfg_with_unsupported_cloud(),
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(
            self,
            out,
            f'The registry data device is not supported with the cloud provider \
                "NonYandex". Please select a registry mode that does \
                not require the registry data device. Available modes: \
                {", ".join(REGISTRY_MODES_WITHOUT_REGISTRY_DATA_DEVICE_SUPPORT)}',
        )

    def test_should_create_with_disabled_registry_data_device_with_unsupported_cloud(
        self,
    ):
        ctx = _prepare_create_binding_context(
            new_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=False
            ),
            cluster_cfg_data=_prepare_cluster_cfg_with_unsupported_cloud(),
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)


class TestProviderSecondaryDevicesConfigurationUpdate(unittest.TestCase):

    def test_should_update_when_switch_to_enable_registry_data_device_with_supported_cloud(
        self,
    ):
        ctx = _prepare_update_binding_context(
            old_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=False
            ),
            new_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=True
            ),
            cluster_cfg_data=_prepare_cluster_cfg_with_supported_cloud(),
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_should_update_when_switch_to_enable_registry_data_device_with_static_cluster(
        self,
    ):
        ctx = _prepare_update_binding_context(
            old_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=False
            ),
            new_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=True
            ),
            cluster_cfg_data=_prepare_cluster_cfg_with_static(),
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_should_update_with_empty_configs(
        self,
    ):
        ctx = _prepare_update_binding_context(
            old_data=None,
            new_data=None,
            cluster_cfg_data=None,
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_should_update_with_empty_fields(
        self,
    ):
        ctx = _prepare_update_binding_context(
            old_data=_prepare_provider_cluster_configuration_with_custom_data(""),
            new_data=_prepare_provider_cluster_configuration_with_custom_data(""),
            cluster_cfg_data="",
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_should_not_update_when_switch_to_enable_registry_data_device_with_empty_cluster_cfg(
        self,
    ):
        ctx = _prepare_update_binding_context(
            old_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=False
            ),
            new_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=True
            ),
            cluster_cfg_data="",
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(
            self, out, "Cannot get provider cluster configuration"
        )

    def test_should_not_update_when_switch_to_enable_registry_data_device_with_unsupported_cloud(
        self,
    ):
        ctx = _prepare_update_binding_context(
            old_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=False
            ),
            new_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=True
            ),
            cluster_cfg_data=_prepare_cluster_cfg_with_unsupported_cloud(),
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_deny(
            self,
            out,
            f'The registry data device is not supported with the cloud provider \
                "NonYandex". Please select a registry mode that does \
                not require the registry data device. Available modes: \
                {", ".join(REGISTRY_MODES_WITHOUT_REGISTRY_DATA_DEVICE_SUPPORT)}',
        )

    def test_should_update_when_switch_to_disable_registry_data_device_with_unsupported_cloud(
        self,
    ):
        ctx = _prepare_update_binding_context(
            old_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=True
            ),
            new_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=False
            ),
            cluster_cfg_data=_prepare_cluster_cfg_with_unsupported_cloud(),
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)


class TestProviderSecondaryDevicesConfigurationDelete(unittest.TestCase):

    def test_should_delete_with_enabled_registry_data_device_with_supported_cloud(
        self,
    ):
        ctx = _prepare_delete_binding_context(
            old_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=True
            ),
            cluster_cfg_data=_prepare_cluster_cfg_with_supported_cloud(),
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_should_delete_with_enabled_registry_data_device_with_static_cluster(
        self,
    ):
        ctx = _prepare_delete_binding_context(
            old_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=True
            ),
            cluster_cfg_data=_prepare_cluster_cfg_with_static(),
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_should_delete_with_empty_configs(
        self,
    ):
        ctx = _prepare_delete_binding_context(
            old_data=None,
            cluster_cfg_data=None,
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_should_delete_with_empty_fields(
        self,
    ):
        ctx = _prepare_delete_binding_context(
            old_data=_prepare_provider_cluster_configuration_with_custom_data(""),
            cluster_cfg_data="",
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_should_delete_with_enabled_registry_data_device_with_empty_cluster_cfg(
        self,
    ):
        ctx = _prepare_delete_binding_context(
            old_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=True
            ),
            cluster_cfg_data="",
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_should_delete_with_enabled_registry_data_device_with_unsupported_cloud(
        self,
    ):
        ctx = _prepare_delete_binding_context(
            old_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=True
            ),
            cluster_cfg_data=_prepare_cluster_cfg_with_unsupported_cloud(),
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)

    def test_should_delete_with_disabled_registry_data_device_with_unsupported_cloud(
        self,
    ):
        ctx = _prepare_delete_binding_context(
            old_data=_prepare_provider_cluster_configuration(
                registry_data_device_enable=False
            ),
            cluster_cfg_data=_prepare_cluster_cfg_with_unsupported_cloud(),
        )
        out = hook.testrun(main, [ctx])
        tests.assert_validation_allowed(self, out, None)


if __name__ == "__main__":
    unittest.main()
