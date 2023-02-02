#!/usr/bin/env python3
#
# Copyright 2023 Flant JSC Licensed under Apache License 2.0
#

import operator
from functools import reduce
from typing import Iterable

from dictdiffer import deepcopy, diff


class ValuesPatchesCollector:
    """
    Wrapper for the values manipulations (JSON patches)
    """

    def __init__(self, values: dict):
        self.initial_values = deepcopy(values)
        self.data = []

    def collect(self, payload: dict):
        self.data.append(payload)

    def update(self, updated_values: dict):
        for patch in values_json_patches(self.initial_values, updated_values):
            self.collect(patch)


def values_json_patches(initial_values: dict, updated_values: dict):
    changes = diff(
        initial_values,
        updated_values,
        dot_notation=False,  # always return path as list
        expand=True,  # do not compact values in single operation
    )
    pg = PatchGenerator(updated_values)
    for change in changes:
        for patch in pg.generate(change):
            yield patch


class PatchGenerator:
    """
    Generates appropriate JSON patches for the dictdiffer changes to be useful in Addon Operator.

    Addon Operator does not permit using "replace" operation, so we use "add" instead. And we treat
    arrays as whole. We have to remove them and set the new value instead of patching them.
    """

    def __init__(self, updated_values: dict):
        self.updated_values = updated_values
        self.seen_array_paths = set()

    def generate(self, change):
        """Generate JSON patches for the dictdiffer change.

        **NOTE**: make sure to pass JSON serizlizable values, e.g. raw dicts, if you use dict
        wrappers like DotMap.

        Args:
            change (dict): the dictdiffer change

        Yields:
            dict: JSON patch dict
        """
        for p in self.__generate(change):
            yield p

    def __generate(self, change):
        """
        Converts dictdiffer change to JSON patches suitable for Addon Operator.
        https://jsonpatch.com/#operations
        """
        op, path_segments, values = change

        if op == "add":
            #   op    |_______path________|   value
            #    |    |                   |  /
            # ('add', ['x', 'y', 'a'], [(2, 2)])

            key, value = values[0]

            if len(path_segments) > 0 and isinstance(key, int):
                # array element
                for p in self.__array_patches(path_segments):
                    yield p
                return

            path = json_path(path_segments + [key])
            yield {"op": "add", "path": path, "value": value}
            return

        if op == "change":
            #   op       |______path______|  from  to
            #    |       |                |   |   /
            # ('change', ['x', 'y', 'a', 0], (1, 0))

            if len(path_segments) > 0 and isinstance(path_segments[-1], int):
                # array element, excluding index from the path
                for p in self.__array_patches(path_segments[:-1]):
                    yield p
                return

            value = values[1]
            path = json_path(path_segments)
            yield {"op": "add", "path": path, "value": value}
            return

        if op == "remove":
            #   op       |______path_____|     value
            #    |       |               |    /
            # ('remove', ['x', 'y'], [('t', 0)])

            key = values[0][0]
            path = json_path(path_segments + [key])
            yield {"op": "remove", "path": path}
            return

        raise ValueError(f"Unknown patch operation: {op}")

    def __array_patches(self, path_segments: Iterable):
        path = json_path(path_segments)

        # avoid duplicate array patches
        if path in self.seen_array_paths:
            return
        self.seen_array_paths.add(path)

        # pick the value by path
        value = reduce(operator.getitem, path_segments, self.updated_values)

        yield {"op": "remove", "path": path}
        yield {"op": "add", "path": path, "value": value}


def json_path(path: Iterable):
    return "/" + "/".join([str(p) for p in path])
