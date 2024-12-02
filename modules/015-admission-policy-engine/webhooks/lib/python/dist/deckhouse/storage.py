#!/usr/bin/env python3
#
# Copyright 2022 Flant JSC Licensed under Apache License 2.0
#

import json


class FileStorage:
    """
    Context manager wrapping the appending JSON per line to file
    """

    def __init__(self, path):
        self.file = open(path, "a", encoding="utf-8")

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        self.file.close()

    def write(self, payload: dict):
        self.file.write(json.dumps(payload))
        self.file.write("\n")
