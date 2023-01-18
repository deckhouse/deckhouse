#!/usr/bin/env python3
#
# Copyright 2022 Flant JSC Licensed under Apache License 2.0
#

import json
import os
import sys
from dataclasses import dataclass
from typing import Iterable

from dictdiffer import deepcopy
from dotmap import DotMap

from .kubernetes import KubeOperationCollector
from .metrics import MetricsCollector
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
    values: DotMap = None

    def __init__(self, metrics, kube_operations, values_patches):
        self.metrics = metrics
        self.kube_operations = kube_operations
        self.values_patches = values_patches

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

        # DotMap for values.dot.notation and config.dot.notation
        # Helm: .Values.moduleName
        # Hook: ctx.config
        self.config_values = dotmapcopy(config_values)
        # Helm: .Values.moduleName.internal
        # Hook: ctx.values
        self.values = dotmapcopy(initial_values)

    @property
    def config(self):
        """Module config derived from module settings and config values schema"""
        if not self.module_name:
            raise ValueError("Module name is not set")
        return self.config_values[self.module_name]

    @property
    def globals(self):
        """Global values

        'global' is a reserved word, so we canot use it"""
        return self.config_values["global"]

    @property
    def internal(self):
        """Internal values"""
        if not self.module_name:
            raise ValueError("Module name is not set")
        return self.values[self.module_name].internal

    @property
    def metrics(self):
        return self.output.metrics

    @property
    def kubernetes(self):
        return self.output.kube_operations

    @property
    def values_patches(self):
        return self.output.values_patches


def dotmapcopy(d: dict):
    return DotMap(deepcopy(d))


def read_binding_context_file():
    """
    Iterates over hook contexts in the binding context file.

    :yield ctx: hook binding context
    """
    context = read_json_file("BINDING_CONTEXT_PATH") or []
    for ctx in context:
        yield ctx


def read_values_file():
    """
    Reads module values from the values file.

    :return values: the dict of the values
    """
    return read_json_file("VALUES_PATH") or {}


def read_config_file():
    """
    Reads module config from the config values file.

    :return values: the dict of the values
    """
    return read_json_file("CONFIG_VALUES_PATH") or {}


def read_json_file(envvar):
    """
    Reads module values from the values file.

    :return values: the dict of the values
    """
    values_path = os.getenv(envvar)
    if not values_path:
        # No values in Shell Operator
        return None
    with open(values_path, "r", encoding="utf-8") as f:
        values = json.load(f)
    return values


def read_module_dirname():
    return os.getenv("D8_MODULE_DIRNAME")


def noprefixnum_camelcase(mod_dir):
    """Translates 123-some-module-name to someModuleName

    Args:
        mod_dir (_type_): the dir name of the module, e.g. 123-some-module-name
    """
    if not mod_dir:
        return ""
    parts = mod_dir.split("-")[1:]
    for i in range(1, len(parts)):
        parts[i] = parts[i].capitalize()
    return "".join(parts)


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
        config_values = DotMap()
    if not initial_values:
        initial_values = DotMap()

    output = Output(
        MetricsCollector(),
        KubeOperationCollector(),
        ValuesPatchesCollector(initial_values),
    )

    for bindctx in binding_context:
        hookctx = Context(bindctx, config_values, initial_values, output, module_name)
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

    binding_context = read_binding_context_file()
    initial_values = read_values_file()
    config_values = read_config_file()
    module_name = noprefixnum_camelcase(read_module_dirname())

    output = __run(func, binding_context, config_values, initial_values, module_name)

    output.flush()


def testrun(
    func,
    binding_context: Iterable = None,
    config_values: dict = None,
    initial_values: dict = None,
    module_name: str = "module",
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
