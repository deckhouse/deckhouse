#!/usr/bin/env python3
#
# Copyright 2022 Flant JSC Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#
import functools
import json
import os
import sys
from dataclasses import dataclass


class KubernetesModifier(object):
    """
    Wrapper for the kubernetes actions: creation, deletion, patching.
    """

    def __init__(self):
        self.file = open(os.getenv("KUBERNETES_PATCH_PATH"), "a", encoding="utf-8")

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        self.file.close()

    def __export(self, payload: dict):
        self.file.write(json.dumps(payload))
        self.file.write("\n")

    def create(self, obj):
        """
        :param obj: must be serializable to JSON
        """
        self.__export({"operation": "Create", "object": obj})

    def create_or_update(self, obj):
        """
        :param obj: must be serializable to JSON
        """
        self.__export({"operation": "CreateOrUpdate", "object": obj})

    def create_if_not_exists(self, obj):
        """
        :param obj: must be serializable to JSON
        """
        self.__export({"operation": "CreateIfNotExists", "object": obj})

    def __delete(
        self, operation, kind, namespace, name, apiVersion=None, subresource=None
    ):
        """
        :param kind: object Kind.
        :param namespace: object namespace. If empty, implies operation on a cluster-level resource.
        :param name: object name.
        :param apiVersion: optional field that specifies object apiVersion. If not present, we'll
            use preferred apiVersion for the given kind.
        :param subresource: a subresource name if subresource is to be transformed. For example,
            status.
        """
        if operation not in ("Delete", "DeleteInBackground", "DeleteNonCascading"):
            raise ValueError(f"Invalid delete operation: {operation}")

        obj = {
            "kind": kind,
            "namespace": namespace,
            "name": name,
        }
        if apiVersion is not None:
            obj["apiVersion"] = apiVersion
        if subresource is not None:
            obj["subresource"] = subresource

        self.__export({"operation": operation, "object": obj})

    def delete(self, kind, namespace, name, apiVersion=None, subresource=None):
        """
        :param kind: object Kind.
        :param namespace: object namespace. If empty, implies operation on a cluster-level resource.
        :param name: object name.
        :param apiVersion: optional field that specifies object apiVersion. If not present, we'll
            use preferred apiVersion for the given kind.
        :param subresource: a subresource name if subresource is to be transformed. For example,
            status.
        """
        return self.__delete("Delete", kind, namespace, name, apiVersion, subresource)

    def delete_in_backgroud(
        self, kind, namespace, name, apiVersion=None, subresource=None
    ):
        """
        :param kind: object Kind.
        :param namespace: object namespace. If empty, implies operation on a cluster-level resource.
        :param name: object name.
        :param apiVersion: optional field that specifies object apiVersion. If not present, we'll
            use preferred apiVersion for the given kind.
        :param subresource: a subresource name if subresource is to be transformed. For example,
            status.
        """
        return self.__delete(
            "DeleteInBackground", kind, namespace, name, apiVersion, subresource
        )

    def delete_non_cascading(
        self, kind, namespace, name, apiVersion=None, subresource=None
    ):
        """
        :param kind: object Kind.
        :param namespace: object namespace. If empty, implies operation on a cluster-level resource.
        :param name: object name.
        :param apiVersion: optional field that specifies object apiVersion. If not present, we'll
            use preferred apiVersion for the given kind.
        :param subresource: a subresource name if subresource is to be transformed. For example,
            status.
        """
        return self.__delete(
            "DeleteNonCascading", kind, namespace, name, apiVersion, subresource
        )

    def merge_patch(
        self,
        kind,
        namespace,
        name,
        patch: dict,
        apiVersion=None,
        subresource=None,
        ignoreMissingObject=False,
    ):
        """
        :param operation: specifies an operation's type.
        :param apiVersion: optional field that specifies object apiVersion. If not present, we'll
            use preferred apiVersion for the given kind.
        :param kind: object Kind.
        :param namespace: object Namespace. If empty, implies operation on a Cluster-level
            resource.
        :param name: object name.
        :param patch: describes transformations to perform on an object. Can be a normal JSON or
            YAML array or a stringified JSON or YAML array.
        :param subresource: a subresource name if subresource is to be transformed. For example,
            status.
        :param ignoreMissingObject: set to true to ignore error when patching non existent object.
        """
        return self.__patch(
            "MergePatch",
            kind,
            namespace,
            name,
            patch,
            apiVersion,
            subresource,
            ignoreMissingObject,
        )

    def json_patch(
        self,
        kind,
        namespace,
        name,
        patch: dict,
        apiVersion=None,
        subresource=None,
        ignoreMissingObject=False,
    ):
        """
        :param apiVersion: optional field that specifies object apiVersion. If not present, we'll
            use preferred apiVersion for the given kind.
        :param kind: object Kind.
        :param namespace: object Namespace. If empty, implies operation on a Cluster-level resource.
        :param name: object name.
        :param patch: describes transformations to perform on an object. Can be a normal JSON or
            YAML array or a stringified JSON or YAML array.
        :param subresource: a subresource name if subresource is to be transformed. For example,
            status.
        :param ignoreMissingObject: set to true to ignore error when patching non existent object.
        """
        return self.__patch(
            "JSONPatch",
            kind,
            namespace,
            name,
            patch,
            apiVersion,
            subresource,
            ignoreMissingObject,
        )

    def __patch(
        self,
        operation,
        kind,
        namespace,
        name,
        patch: dict,
        apiVersion=None,
        subresource=None,
        ignoreMissingObject=False,
    ):
        """
        :param operation: specifies an operation's type.
        :param apiVersion: optional field that specifies object apiVersion. If not present, we'll
            use preferred apiVersion for the given kind.
        :param kind: object Kind.
        :param namespace: object Namespace. If empty, implies operation on a Cluster-level resource.
        :param name: object name.
        :param patch: describes transformations to perform on an object. Can be a normal JSON or
            YAML array or a stringified JSON or YAML array.
        :param subresource: a subresource name if subresource is to be transformed. For example,
            status.
        :param ignoreMissingObject: set to true to ignore error when patching non existent object.
        """
        ret = {
            "operation": operation,
            "kind": kind,
            "namespace": namespace,
            "name": name,
        }
        if operation == "MergePatch":
            ret["mergePatch"] = patch
        elif operation == "JSONPatch":
            ret["jsonPatch"] = patch
        else:
            raise ValueError(f"Invalid patch operation: {operation}")

        if apiVersion is not None:
            ret["apiVersion"] = apiVersion
        if subresource is not None:
            ret["subresource"] = subresource
        if ignoreMissingObject:
            ret["ignoreMissingObject"] = ignoreMissingObject

        self.__export(ret)


class MetricsExporter(object):
    """
    Wrapper for metrics exporting. Accepts raw dicts and appends them into the metrics file.
    """

    def __init__(self):
        self.file = open(os.getenv("METRICS_PATH"), "a", encoding="utf-8")

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        self.file.close()

    def export(self, metric: dict):
        self.file.write(json.dumps(metric))
        self.file.write("\n")

    def expire_group(self, metric_group: str):
        self.export({"action": "expire", "group": metric_group})


def read_binding_context():
    """
    Itrates over hook contexts in the binding context file.

    Yields:
        _type_: dict
    """
    context_path = os.getenv("BINDING_CONTEXT_PATH")
    with open(context_path, "r", encoding="utf-8") as f:
        context = json.load(f)
    for ctx in context:
        yield ctx


def bindingcontext(configpath):
    """
    Provides binding context for hook.

    Example:

     for ctx in bindingcontext("node_metrics.yaml")
        do_something(ctx)
    """
    if len(sys.argv) > 1 and sys.argv[1] == "--config":
        with open(configpath, "r", encoding="utf-8") as cf:
            print(cf.read())
            sys.exit(0)

    for ctx in read_binding_context():
        hook_ctx = HookContext(
            binding_context=ctx,
            snapshots=ctx.get("snapshots", {}),
            metrics=MetricsExporter(),
            kubernetes=KubernetesModifier(),
        )
        yield hook_ctx


# TODO --log-proxy-hook-json / LOG_PROXY_HOOK_JSON (default=false)
#   Delegate hook stdout/ stderr JSON logging to the hooks and act as a proxy that adds some extra #
#   fields before just printing the output. NOTE: It ignores LOG_TYPE for the output of the hooks; #
#   expects JSON lines to stdout/ stderr from the hooks


@dataclass
class HookContext:
    binding_context: dict
    snapshots: list
    metrics: MetricsExporter
    kubernetes: KubernetesModifier


def run(func, configpath):
    for ctx in bindingcontext(configpath):
        func(ctx)
