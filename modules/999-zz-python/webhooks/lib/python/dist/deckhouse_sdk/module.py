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


def get_root():
    return os.getenv("D8_MODULE_ROOT") or ""


def get_name():
    mod_root = get_root()
    _, mod_dir = os.path.split(mod_root)
    return noprefixnum_camelcase(mod_dir)


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
