#!/usr/bin/env python3
#
# Copyright 2023 Flant JSC Licensed under Apache License 2.0
#


class ConversionsCollector:
    """
    Wrapper for the conversions feature of Shell Operator.

    https://github.com/flant/shell-operator/blob/main/BINDING_CONVERSION.md
    """

    def __init__(self):
        self._converted_objects = []
        self._err_message = None

    def collect(self, payload: dict):
        self._converted_objects.append(payload)

    @property
    def data(self):
        """The data is a list of ONLY ONE object, because this object will be a single JSON
        conversion response.

        Returns:
            list: the list of single response
        """
        if self._err_message is not None:
            return [{"failedMessage": self._err_message}]
        return [{"convertedObjects": self._converted_objects}]

    def error(self, message: str):
        """Overwrites all previous data with a single error message.

        Args:
            message (str): error message
        """
        self._err_message = message
