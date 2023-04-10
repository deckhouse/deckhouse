#!/usr/bin/env python3
#
# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#
# This hook is responsible for generating metrics for d8 controllers resource consumption.


from utils import prometheus_query_value, prometheus_metric_builder, prometheus_function_builder, check_for_generate_mock
from typing import List, Dict, Any, Optional
from dataclasses import dataclass
from shell_operator import hook
from pathlib import Path
import json
import os


RESOURCE_CONSUMPTION_QUERY = '''
sum (
    ( {} )
    + on(pod) group_left(controller_name, controller_type)
    ( {} * 0 )
)
'''
PROMETHEUS_INTERVAL = "5m"


PROMETHEUS_METRIC_GROUP = "group_d8_controller_metrics"
PROMETHEUS_CPU_METRIC_NAME = "deckhouse_telemetry_controller_average_cpu_usage_seconds"
PROMETHEUS_MEMORY_METRIC_NAME = "deckhouse_telemetry_controller_average_memory_working_set_bytes:without_kmem"

MOCK_DATA_FILE = None
if check_for_generate_mock():
    MOCK_DATA_FILE = os.path.join(os.path.dirname(__file__), f"{Path(__file__).stem}_mock_data.json")
    with open(MOCK_DATA_FILE, "w") as f:
        json.dump([], f)


@dataclass
class Controller:
    name: str
    namespace: str
    kind: str
    module: str
    cpu: Optional[float] = None
    memory: Optional[float] = None


def main(ctx: hook.Context):
    print(json.dumps(ctx.snapshots, indent=2))
    # Generate list of Controllers from snapshots
    controllers = process_controllers(snapshots=ctx.snapshots)

    # Generate metrics from Controllers list
    metrics = generate_metrics(controllers=controllers)

    # Export metrics
    ctx.metrics.expire(group=PROMETHEUS_METRIC_GROUP)
    for metric in metrics:
        ctx.metrics.collect(metric)


def generate_metrics(controllers: List[Controller]) -> List[Dict[str, Any]]:
    '''Generate metrics from Controllers list'''

    metrics = []
    for ctrl in controllers:
        metrics.append({
            "name": PROMETHEUS_CPU_METRIC_NAME,
            "group": PROMETHEUS_METRIC_GROUP,
            "set": ctrl.cpu,
            "labels": controller_metric_labels(ctrl=ctrl),
        })
        metrics.append({
            "name": PROMETHEUS_MEMORY_METRIC_NAME,
            "group": PROMETHEUS_METRIC_GROUP,
            "set": ctrl.memory,
            "labels": controller_metric_labels(ctrl=ctrl),
        })
    return metrics


def controller_metric_labels(ctrl: Controller):
    '''Helper func with labels for resource metrics'''

    return {
        "controller_name": ctrl.name,
        "controller_module": ctrl.module,
        "controller_kind": ctrl.kind,
    }


def process_controllers(snapshots: Dict[str, List[Dict[str, Any]]]) -> List[Controller]:
    '''Generate list of Controllers from binding context snapshots'''

    return [parse_controller(controller_snapshot=sn) for queue_snapshot in snapshots.values() for sn in queue_snapshot]


def parse_controller(controller_snapshot: Dict[str, Any]) -> Controller:
    '''
    Generate controller from snapshot and
    query prometheus for its memory and cpu consumption
    '''

    filter_result = controller_snapshot["filterResult"]
    controller = Controller(
        kind=filter_result["controller_kind"],
        name=filter_result["controller_name"],
        namespace=filter_result["controller_namespace"],
        module=filter_result["controller_module"],
    )
    cpu_mock_data, mem_mock_data = None, None
    if check_for_generate_mock():
        mock_data = {
            "group": PROMETHEUS_METRIC_GROUP,
            "labels": controller_metric_labels(ctrl=controller),
        }
        cpu_mock_data = dict(**mock_data, name=PROMETHEUS_CPU_METRIC_NAME)
        mem_mock_data = dict(**mock_data, name=PROMETHEUS_MEMORY_METRIC_NAME)

    controller.cpu = get_cpu_prometheus(
        controller=controller,
        mock_data=cpu_mock_data,
        mock_data_file=MOCK_DATA_FILE,
    )
    controller.memory = get_memory_prometheus(
        controller=controller,
        mock_data=mem_mock_data,
        mock_data_file=MOCK_DATA_FILE,
    )
    return controller


def get_cpu_prometheus(controller: Controller, mock_data: Any, mock_data_file: str | None):
    '''Query prometheus for controller cpu consumption'''

    return prometheus_query_value(
        query=RESOURCE_CONSUMPTION_QUERY.format(
            prometheus_function_builder(
                f="rate",
                metric=resource_metric("container_cpu_usage_seconds_total", controller),
                interval=PROMETHEUS_INTERVAL,
            ),
            controller_metric(controller),
        ),
        addtitional_mock_data=mock_data,
        mock_data_file=mock_data_file,
    )


def get_memory_prometheus(controller: Controller, mock_data: Any, mock_data_file: str | None):
    '''Query prometheus for controller memory consumption'''

    return prometheus_query_value(
        query=RESOURCE_CONSUMPTION_QUERY.format(
            prometheus_function_builder(
                f="avg_over_time",
                metric=resource_metric("container_memory_working_set_bytes:without_kmem", controller),
                interval=PROMETHEUS_INTERVAL,
            ),
            controller_metric(controller),
        ),
        addtitional_mock_data=mock_data,
        mock_data_file=mock_data_file,
    )


def controller_metric(controller: Controller) -> str:
    '''
    Generate kube_controller_pod metric from Controller instance
    input: Controller(name="dex", namespace="d8-user-authn, kind="Deployment")
    output: kube_controller_pod{namespace="d8-user-authn",controller_name="dex",controller_type="Deployment"}
    '''

    return prometheus_metric_builder(
        metric_name="kube_controller_pod",
        labels={
            "namespace": controller.namespace,
            "controller_name": controller.name,
            "controller_type": controller.kind
        },
    )


def resource_metric(metric_name: str, controller: Controller):
    '''
    Generate resource metric from Controller instance and metric name
    input: "container_cpu_usage_seconds_total", Controller(name="dex", namespace="d8-user-authn, kind="Deployment")
    output: `container_cpu_usage_seconds_total{namespace="d8-user-authn"}`
    '''

    return prometheus_metric_builder(
        metric_name=metric_name,
        labels={
            "namespace": controller.namespace,
        },
    )


if __name__ == "__main__":
    hook.run(main, configpath="controller_metrics.yaml")
