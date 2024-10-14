#!/usr/bin/env python3
#
# Copyright 2023 Flant JSC Licensed under Apache License 2.0
#


import typing


class ValidationsCollector:
    """
    Wrapper for the validating feature of Shell Operator.

    https://github.com/flant/shell-operator/blob/main/BINDING_VALIDATING.md
    """

    def __init__(self):
        self._data = []

    def collect(self, payload: dict):
        self._data.append(payload)

    @property
    def data(self):
        """The data is a list of ONLY ONE object, because this object will be a single JSON
        conversion response.

        Returns:
            list: the list of single response
        """
        return self._data

    def allow(self, *warnings: str):
        response = {"allowed": True}
        if len(warnings) > 0:
            response["warnings"] = warnings
        self.collect(response)

    def deny(self, message: typing.Union[str, None] = None):
        response = {"allowed": False}
        if message is not None:
            response["message"] = message
        self.collect(response)

    def error(self, message: str):
        self.collect({"allowed": False, message: f"Internal webhook error: {message}"})
