#!/usr/bin/env python3
#
# Copyright 2023 Flant JSC Licensed under Apache License 2.0
#


import json
import os


def get_binding_context():
    """
    Iterates over hook contexts in the binding context file.

    :yield ctx: hook binding context
    """
    context = read_json_file("BINDING_CONTEXT_PATH") or []
    for ctx in context:
        yield ctx


def get_values():
    """
    Reads module values from the values file.

    :return values: the dict of the values
    """
    return read_json_file("VALUES_PATH") or {}


def get_config():
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
