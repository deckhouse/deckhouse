#!/usr/bin/env python3
#
# Copyright 2022 Flant JSC Licensed under Apache License 2.0
#


import os
import sys
from dataclasses import dataclass
from typing import Iterable

from dictdiffer import deepcopy

from .conversions import ConversionsCollector
from .kubernetes import KubeOperationCollector
from .metrics import MetricsCollector
from .module import get_binding_context, get_config
from .module import get_name as get_module_name
from .module import get_values
from .storage import FileStorage
from .values import ValuesPatchesCollector


class Output:
    """
    Container for output means for metrics, kubernetes, and values.

    Metrics, Kubernetes JSON patches and values JSON patches are collected in underlying storages,
    whether shell-operator (or addon-operator) file paths, or into memory.
    """

    # Values with outputs for tests, values patches are less convenient than values
    # themselves.
    values: dict = None

    def __init__(
        self,
        metrics: MetricsCollector,
        kube_operations: KubeOperationCollector,
        values_patches: ValuesPatchesCollector,
        conversions: ConversionsCollector,
    ):
        self.metrics = metrics
        self.kube_operations = kube_operations
        self.values_patches = values_patches
        self.conversions = conversions

    # TODO  logger: --log-proxy-hook-json / LOG_PROXY_HOOK_JSON (default=false)
    #
    # Delegate hook stdout/stderr JSON logging to the hooks and act as a proxy that adds some extra
    # fields before just printing the output. NOTE: It ignores LOG_TYPE for the output of the hooks;
    # expects JSON lines to stdout/stderr from the hooks

    def flush(self):
        file_outputs = (
            ("METRICS_PATH", self.metrics),
            ("KUBERNETES_PATCH_PATH", self.kube_operations),
            ("VALUES_JSON_PATCH_PATH", self.values_patches),
            ("CONVERSION_RESPONSE_PATH", self.conversions),
        )

        for path_env, collector in file_outputs:
            path = os.getenv(path_env)
            if not path:
                # No values in Shell Operator
                continue
            with FileStorage(path) as file:
                for payload in collector.data:
                    file.write(payload)


@dataclass
class Context:
    def __init__(
        self,
        binding_context: dict,
        config_values: dict,
        initial_values: dict,
        output: Output,
        module_name: str,
    ):
        self.binding_context = binding_context
        self.snapshots = binding_context.get("snapshots", {})
        self.output = output
        self.module_name = module_name
        self.config_values = deepcopy(config_values)
        self.values = deepcopy(initial_values)

    @property
    def metrics(self):
        return self.output.metrics

    @property
    def kubernetes(self):
        return self.output.kube_operations

    @property
    def values_patches(self):
        return self.output.values_patches


def __run(
    func,
    binding_context: list,
    config_values: dict,
    initial_values: dict,
    module_name: str,
):
    """
    Run the hook function with config. Accepts config path or config text.

    :param func: the function to run
    :param binding_context: the list of hook binding contexts
    :param config_values: config values
    :param initial_values: initial values
    :param module_name: module name in camelCase
    :return output: output means with all generated payloads and updated values
    """

    if not binding_context:
        binding_context = [{}]
    if not config_values:
        config_values = {}
    if not initial_values:
        initial_values = {}

    output = Output(
        MetricsCollector(),
        KubeOperationCollector(),
        ValuesPatchesCollector(initial_values),
        ConversionsCollector(),
    )

    for bindctx in binding_context:
        hookctx = Context(
            binding_context=bindctx,
            config_values=config_values,
            initial_values=initial_values,
            output=output,
            module_name=module_name,
        )
        func(hookctx)
        output.values = hookctx.values
        output.values_patches.update(hookctx.values)

    return output


def run(func, configpath=None, config=None):
    """
    Run the hook function with config. Accepts config path or config text.

    :param configpath: path to the hook config file
    :param config: hook config text itself
    """

    if len(sys.argv) > 1 and sys.argv[1] == "--config":
        if config is None and configpath is None:
            raise ValueError("config or configpath must be provided")

        if config is not None:
            print(config)
        else:
            with open(configpath, "r", encoding="utf-8") as cf:
                print(cf.read())

        sys.exit(0)

    output = __run(
        func,
        binding_context=get_binding_context(),
        config_values=get_values(),
        initial_values=get_config(),
        module_name=get_module_name(),
    )

    output.flush()


def testrun(
    func,
    binding_context: Iterable = None,
    config_values: dict = None,
    initial_values: dict = None,
    module_name: str = get_module_name(),
) -> Output:
    """
    Test-run the hook function. Accepts binding context and initial values.

    Returns output means for metrics, kubernetes, values patches, and also modified values for more
    convenient tests.

    :param binding_context: the list of hook binding contexts
    :param initial_values: initial values
    :return: output means for metrics and kubernetes
    """

    output = __run(func, binding_context, config_values, initial_values, module_name)
    return output
