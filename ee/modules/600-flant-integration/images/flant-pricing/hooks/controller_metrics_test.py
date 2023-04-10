#!/usr/bin/env python3

# Copyright 2023
# Flant JSC Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

from controller_metrics import main
from shell_operator import hook
from typing import List, Dict, Any
from utils import check_for_generate_mock
import json
import os
import httpretty


def _generate_mock_prom_server() -> List[Dict[str, Any]]:
    expected_metrics = [{'action': 'expire', 'group': 'group_d8_controller_metrics'}]
    with open(os.path.join(os.path.dirname(__file__), "controller_metrics_mock_data.json"), "r") as f:
        data = json.load(f)

    for d in data:
        httpretty.register_uri(uri=d["uri"], body=d["data"], method="GET", match_querystring=True)

        metric_data = d["addtitional_data"]
        metric_value = json.loads(d["data"])["data"]["result"][0]["value"][1]
        expected_metrics.append(dict(**metric_data, set=metric_value))
    return expected_metrics


def test_controller_metrics():
    expected_metrics = []
    if not check_for_generate_mock():
        httpretty.enable(verbose=True, allow_net_connect=False)
        expected_metrics = _generate_mock_prom_server()

    out = hook.testrun(main, binding_context)

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
                        "controller_name": "dex",
                        "controller_module": "user-authn",
                        "controller_kind": "Deployment",
                        "controller_namespace": "d8-user-authn",
                    }
                },
            ],
            "ds": [
                {
                    "filterResult": {
                        "controller_name": "controller-nginx",
                        "controller_module": "ingress-nginx",
                        "controller_kind": "DaemonSet",
                        "controller_namespace": "d8-ingress-nginx",
                    }
                },
            ],
            "sts": [
                {
                    "filterResult": {
                        "controller_name": "prometheus-main",
                        "controller_module": "monitoring",
                        "controller_kind": "StatefulSet",
                        "controller_namespace": "d8-monitoring",
                    }
                },
                {
                    "filterResult": {
                        "controller_name": "openvpn",
                        "controller_module": "openvpn",
                        "controller_kind": "StatefulSet",
                        "controller_namespace": "d8-openvpn",
                    }
                },
            ],
        },
    }
]
