# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#
# util functions for hook.

from requests.adapters import Retry, PoolManager, HTTPResponse, ResponseError
from urllib.parse import urlencode
from urllib3 import disable_warnings
from typing import Dict, List, Any
import ssl
import json
import os
import sys

disable_warnings()

BACKOFF_LIMIT = 3

PROMETHEUS_URL = "https://prometheus.d8-monitoring:9090/api/v1/query"
REQUEST_ARGS = {}


def check_for_generate_mock() -> bool:
    """Check if mock data should be generated"""
    return os.getenv("SHELL_OPERATOR_GET_DATA_FOR_MOCKS") == "yes" and "pytest" in sys.modules


try:
    with open("/var/run/secrets/kubernetes.io/serviceaccount/token") as f:
        SERVICE_ACCOUNT_TOKEN = f.read()
except FileNotFoundError:
    SERVICE_ACCOUNT_TOKEN = os.getenv("SERVICE_ACCOUNT_TOKEN")


def prometheus_metric_builder(metric_name: str, labels: Dict[str, str] = None):
    """build metric from metric name and labels"""

    metric_labels = []
    if labels:
        metric_labels = [f'{k}="{v}"' for k, v in labels.items()]
    return f"{metric_name}{{{','.join(metric_labels)}}}"


def prometheus_function_builder(f: str, metric: str, interval: str = None) -> str:
    """build prometheus function from func, metric name and internval"""

    interval_str = ""
    if interval is not None:
        interval_str = f'[{interval}]'

    return f'{f}({metric}{interval_str})'


def prometheus_query(query: str, addtitional_mock_data: Any = None, mock_data_file: str | None = None) -> List[Dict[str, Any]]:
    """query prometheus from query and get api result response"""

    response = make_get_request(
        url=PROMETHEUS_URL,
        params={"query": query},
        headers={"Authorization": f"Bearer {SERVICE_ACCOUNT_TOKEN}"},
        decode_json=True,
        addtitional_mock_data=addtitional_mock_data,
        mock_data_file=mock_data_file,
    )
    # response:
    # {"status": "success", "data": {"resultType": "vector", "result": [{"metric": {"__name__": "name"}, "value": ["timestamp", "value"]}]}}
    if response["status"] != "success":
        raise ResponseError(f"error quering prometheus with query '{query}'")

    return response.get("data", {}).get("result", [0])


def prometheus_query_value(query: str, addtitional_mock_data: Any = None, mock_data_file: str | None = None) -> int:
    """query prometheus from query and get only value result"""

    query_result = prometheus_query(
        query=query,
        addtitional_mock_data=addtitional_mock_data,
        mock_data_file=mock_data_file,
    )
    if len(query_result) > 0:
        return query_result[0].get("value", [0, 0])[1]
    return 0


def make_get_request(
        url: str,
        params: Dict[str, str] = None,
        headers: Dict[str, str] = None,
        decode_json: bool = False,
        addtitional_mock_data: Any = None,
        mock_data_file: str | None = None,
) -> HTTPResponse:
    """make http request with retries"""

    retries = Retry(
        total=BACKOFF_LIMIT,
        backoff_factor=0.1,
        status_forcelist=[500, 502, 503, 504],
    )
    http = PoolManager(retries=retries, cert_reqs=ssl.CERT_NONE)
    response = http.request("GET", url, fields=params,
                            headers=headers)

    if response.status != 200:
        raise ResponseError(
            f"{response.status} status code: {response.data.decode('utf-8')}"
        )

    if check_for_generate_mock():
        save_mock_data(
            mock_data_file=mock_data_file,
            data={
                "uri": f"{url}?{urlencode(params)}",
                "data": response.data.decode(),
                "query": params,
                "addtitional_data": addtitional_mock_data,
            },
        )

    if decode_json:
        return json.loads(response.data)

    return response.data


def save_mock_data(mock_data_file: str, data: Dict[str, Any]):
    """save data to mock file"""

    with open(mock_data_file, "r") as f:
        old_data = json.load(f)

    with open(mock_data_file, "w") as f:
        old_data.append(data)
        json.dump(old_data, f, indent=4)
