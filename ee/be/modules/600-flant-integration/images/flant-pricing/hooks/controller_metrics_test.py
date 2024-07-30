#!/usr/bin/env python3
#
# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
# See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#

from controller_metrics import HookRunner, AbstractMetricCollector, Controller
from shell_operator import hook


class MockMetricCollector(AbstractMetricCollector):
    __cpu_values = {
        # controller, module, kind, namespace
        ("dex", "user-authn", "Deployment", "d8-user-authn"): 1.0,
        ("controller-nginx", "ingress-nginx", "DaemonSet", "d8-ingress-nginx"): 3.5,
        ("prometheus-main", "monitoring", "StatefulSet", "d8-monitoring"): 2.0,
        ("openvpn", "openvpn", "StatefulSet", "d8-openvpn"): 2.5,
    }

    __memory_values = {
        # controller, module, kind, namespace
        ("dex", "user-authn", "Deployment", "d8-user-authn"): 100,
        ("controller-nginx", "ingress-nginx", "DaemonSet", "d8-ingress-nginx"): 500,
        ("prometheus-main", "monitoring", "StatefulSet", "d8-monitoring"): 10000,
        ("openvpn", "openvpn", "StatefulSet", "d8-openvpn"): 150,
    }

    def get_cpu_controller_consumption(self, controller: Controller) -> float:
        return self.__cpu_values[
            (controller.name, controller.module, controller.kind, controller.namespace)
        ]

    def get_memory_controller_consumption(self, controller: Controller) -> float:
        return self.__memory_values[
            (controller.name, controller.module, controller.kind, controller.namespace)
        ]


def test_controller_metrics():
    hook_runner = HookRunner(MockMetricCollector())
    out = hook.testrun(hook_runner.run, binding_context)

    assert out.metrics.data == expected_metrics
    assert not out.kube_operations.data


binding_context = [
    {
        "binding": "main",
        "type": "Group",
        "snapshots": {
            "deploy": [
                {
                    "filterResult": {
                        "name": "dex",
                        "module": "user-authn",
                        "kind": "Deployment",
                        "namespace": "d8-user-authn",
                    }
                },
            ],
            "ds": [
                {
                    "filterResult": {
                        "name": "controller-nginx",
                        "module": "ingress-nginx",
                        "kind": "DaemonSet",
                        "namespace": "d8-ingress-nginx",
                    }
                },
            ],
            "sts": [
                {
                    "filterResult": {
                        "name": "prometheus-main",
                        "module": "monitoring",
                        "kind": "StatefulSet",
                        "namespace": "d8-monitoring",
                    }
                },
                {
                    "filterResult": {
                        "name": "openvpn",
                        "module": "openvpn",
                        "kind": "StatefulSet",
                        "namespace": "d8-openvpn",
                    }
                },
            ],
        },
    }
]


expected_metrics = [
    {"action": "expire", "group": "group_d8_controller_metrics"},
    {
        "name": "flant_pricing_controller_average_cpu_usage_seconds",
        "group": "group_d8_controller_metrics",
        "set": 1.0,
        "labels": {
            "name": "dex",
            "module": "user-authn",
            "kind": "Deployment",
        },
    },
    {
        "name": "flant_pricing_controller_average_memory_working_set_bytes:without_kmem",
        "group": "group_d8_controller_metrics",
        "set": 100,
        "labels": {
            "name": "dex",
            "module": "user-authn",
            "kind": "Deployment",
        },
    },
    {
        "name": "flant_pricing_controller_average_cpu_usage_seconds",
        "group": "group_d8_controller_metrics",
        "set": 3.5,
        "labels": {
            "name": "controller-nginx",
            "module": "ingress-nginx",
            "kind": "DaemonSet",
        },
    },
    {
        "name": "flant_pricing_controller_average_memory_working_set_bytes:without_kmem",
        "group": "group_d8_controller_metrics",
        "set": 500,
        "labels": {
            "name": "controller-nginx",
            "module": "ingress-nginx",
            "kind": "DaemonSet",
        },
    },
    {
        "name": "flant_pricing_controller_average_cpu_usage_seconds",
        "group": "group_d8_controller_metrics",
        "set": 2.0,
        "labels": {
            "name": "prometheus-main",
            "module": "monitoring",
            "kind": "StatefulSet",
        },
    },
    {
        "name": "flant_pricing_controller_average_memory_working_set_bytes:without_kmem",
        "group": "group_d8_controller_metrics",
        "set": 10000,
        "labels": {
            "name": "prometheus-main",
            "module": "monitoring",
            "kind": "StatefulSet",
        },
    },
    {
        "name": "flant_pricing_controller_average_cpu_usage_seconds",
        "group": "group_d8_controller_metrics",
        "set": 2.5,
        "labels": {
            "name": "openvpn",
            "module": "openvpn",
            "kind": "StatefulSet",
        },
    },
    {
        "name": "flant_pricing_controller_average_memory_working_set_bytes:without_kmem",
        "group": "group_d8_controller_metrics",
        "set": 150,
        "labels": {
            "name": "openvpn",
            "module": "openvpn",
            "kind": "StatefulSet",
        },
    },
]
