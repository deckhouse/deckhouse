#!/usr/bin/env python3
#
# Copyright 2022 Flant JSC Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#
import json
import os
import sys
from contextlib import contextmanager


@contextmanager
def bindingcontext(configpath):
    """
    Provides binding context for hook.
    """
    # Hook config
    if len(sys.argv) > 1 and sys.argv[1] == "--config":
        with open(configpath, "r", encoding="utf-8") as cf:
            print(cf.read())
            exit(0)

    yield read_binding_context()


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


def read_binding_context():
    i = os.getenv("BINDING_CONTEXT_CURRENT_INDEX")
    if i is None:
        i = 0
    else:
        i = int(i)

    context_path = os.getenv("BINDING_CONTEXT_PATH")
    with open(context_path, "r", encoding="utf-8") as f:
        context = json.load(f)
    return context[i]
