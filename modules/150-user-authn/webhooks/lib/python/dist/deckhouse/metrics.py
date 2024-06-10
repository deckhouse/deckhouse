#!/usr/bin/env python3
#
# Copyright 2022 Flant JSC Licensed under Apache License 2.0
#


class MetricsCollector:
    """
    Wrapper for metrics exporting. Accepts raw dicts and appends them into the metrics file.
    """

    def __init__(self):
        self.data = []

    def collect(self, payload: dict):
        self.data.append(payload)

    def expire(self, group: str):
        """Expire all metrics in the group.

        Args:
            group (str): metric group name
        """
        self.collect({"action": "expire", "group": group})
