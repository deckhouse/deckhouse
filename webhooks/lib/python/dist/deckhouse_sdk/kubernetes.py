#!/usr/bin/env python3
#
# Copyright 2022 Flant JSC Licensed under Apache License 2.0
#


class KubeOperationCollector:
    """
    Wrapper for the kubernetes actions: creation, deletion, patching.
    """

    def __init__(self):
        self.data = []

    def collect(self, payload: dict):
        self.data.append(payload)

    def create(self, obj):
        """
        :param obj: must be serializable to JSON
        """
        self.collect({"operation": "Create", "object": obj})

    def create_or_update(self, obj):
        """
        :param obj: must be serializable to JSON
        """
        self.collect({"operation": "CreateOrUpdate", "object": obj})

    def create_if_not_exists(self, obj):
        """
        :param obj: must be serializable to JSON
        """
        self.collect({"operation": "CreateIfNotExists", "object": obj})

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

        self.collect({"operation": operation, "object": obj})

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

        self.collect(ret)
