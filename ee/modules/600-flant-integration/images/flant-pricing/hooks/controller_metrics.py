#!/usr/bin/env python3
#
# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
# See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#
# This hook is responsible for generating metrics for d8 controllers resource consumption.

from os import getenv
from dataclasses import dataclass
from abc import ABC, abstractmethod
from typing import List, Dict, Any, TypeVar

from shell_operator import hook
from utils import MetricQuerierT, PrometheusQuerier


@dataclass
class Controller:
    name: str
    namespace: str
    kind: str
    module: str


MetricCollectorT = TypeVar("MetricCollectorT", bound="AbstractMetricCollector")


class AbstractMetricCollector(ABC):
    metric_group = "group_d8_controller_metrics"
    cpu_metric_name = "flant_pricing_controller_average_cpu_usage_seconds"
    memory_metric_name = (
        "flant_pricing_controller_average_memory_working_set_bytes:without_kmem"
    )

    def collect(self, ctx: hook.Context, controllers: List[Controller]):
        """Export metrics to hook context from Controllers list"""

        ctx.metrics.expire(self.metric_group)
        for ctrl in controllers:
            # Export metrics
            labels = {
                "name": ctrl.name,
                "module": ctrl.module,
                "kind": ctrl.kind,
            }
            ctx.metrics.collect(
                {
                    "name": self.cpu_metric_name,
                    "group": self.metric_group,
                    "set": self.get_cpu_controller_consumption(ctrl),
                    "labels": labels,
                }
            )

            ctx.metrics.collect(
                {
                    "name": self.memory_metric_name,
                    "group": self.metric_group,
                    "set": self.get_memory_controller_consumption(ctrl),
                    "labels": labels,
                }
            )

    @abstractmethod
    def get_cpu_controller_consumption(self, controller: Controller) -> float:
        raise NotImplementedError(
            "define get_cpu_controller_consumption to use this base class"
        )

    @abstractmethod
    def get_memory_controller_consumption(self, controller: Controller) -> float:
        raise NotImplementedError(
            "define get_memory_controller_consumption to use this base class"
        )


class MetricCollector(AbstractMetricCollector):
    def __init__(self, querier: MetricQuerierT):
        super().__init__()
        self.querier = querier

    def get_cpu_controller_consumption(self, controller: Controller) -> float:
        """Query prometheus for controller cpu consumption"""

        metric_name = "container_cpu_usage_seconds_total"
        func = "rate"

        return self.consumption_query(func, metric_name, controller)

    def get_memory_controller_consumption(self, controller: Controller) -> float:
        """Query prometheus for controller memory consumption"""

        metric_name = "container_memory_working_set_bytes:without_kmem"
        func = "avg_over_time"

        return self.consumption_query(func, metric_name, controller)

    def consumption_query(
        self, func: str, metric_name: str, controller: Controller
    ) -> float:
        """Query prometheus for controller resource consumption"""

        query = f"""
            avg (
                sum by (pod) (
                    ( {func}({metric_name}{{
                        namespace="{controller.namespace}"
                    }}[5m]) )

                    + on(pod) group_left(controller_name, controller_type)

                    ( kube_controller_pod{{
                        namespace="{controller.namespace}",
                        controller_name="{controller.name}",
                        controller_type="{controller.kind}"
                    }} * 0 )
                )
            )
        """

        return self.querier.query_value(query)


class HookRunner:
    def __init__(self, collector: MetricCollectorT):
        self.metric_collector = collector

    def run(self, ctx: hook.Context):
        """Run shell operator hook"""
        # Generate list of Controllers from snapshots
        controllers = self.__process_controllers(ctx.snapshots)

        # Generate metrics from Controllers list
        self.metric_collector.collect(ctx, controllers)

    def __process_controllers(
        self, snapshots: Dict[str, List[Dict[str, Any]]]
    ) -> List[Controller]:
        """Generate list of Controllers from binding context snapshots"""

        controllers = []
        for queue_snapshot in snapshots.values():
            for snapshot in queue_snapshot:
                controllers.append(self.__parse_controller(snapshot))
        return controllers

    def __parse_controller(self, controller_snapshot: Dict[str, Any]) -> Controller:
        """
        Generate controller instance from snapshot
        """

        filter_result = controller_snapshot["filterResult"]
        return Controller(
            kind=filter_result["kind"],
            name=filter_result["name"],
            namespace=filter_result["namespace"],
            module=filter_result["module"],
        )


if __name__ == "__main__":
    try:
        with open(
            "/var/run/secrets/kubernetes.io/serviceaccount/token", encoding="utf-8"
        ) as f:
            service_account_token = f.read()
    except FileNotFoundError:
        service_account_token = (
            token if (token := getenv("SERVICE_ACCOUNT_TOKEN")) else ""
        )

    prometheus_querier = PrometheusQuerier(service_account_token)
    metric_collector = MetricCollector(prometheus_querier)
    hook_runner = HookRunner(metric_collector)
    hook.run(hook_runner.run, configpath="controller_metrics.yaml")
