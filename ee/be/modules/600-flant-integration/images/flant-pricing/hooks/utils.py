# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
# See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
#
# util functions for hook.

from ssl import CERT_NONE
from json import loads
from typing import Dict, List, Any, TypeVar, Optional
from abc import ABC, abstractmethod
from urllib3 import disable_warnings
from requests.adapters import Retry, PoolManager, ResponseError

disable_warnings()

MetricQuerierT = TypeVar("MetricQuerierT", bound="AbstractMetricQuerier")


class AbstractMetricQuerier(ABC):
    backoff_limit = 3

    def do_get_request(
        self,
        url: str,
        params: Optional[Dict[str, str]] = None,
        headers: Optional[Dict[str, str]] = None,
        decode_json: bool = False,
    ) -> Any:
        """do http request with retries"""

        retries = Retry(
            total=self.backoff_limit,
            backoff_factor=0.1,
            status_forcelist=[500, 502, 503, 504],
        )
        pool = PoolManager(retries=retries, cert_reqs=CERT_NONE)
        response = pool.request("GET", url, fields=params, headers=headers)

        if response.status != 200:
            raise ResponseError(
                f"{response.status} status code: {response.data.decode('utf-8')}"
            )

        if decode_json:
            return loads(response.data)

        return response.data

    @abstractmethod
    def query(self, query: str) -> List[Dict[str, Any]]:
        raise NotImplementedError("define query to use this base class")

    @abstractmethod
    def query_value(self, query: str) -> float:
        raise NotImplementedError("define query_value to use this base class")


class PrometheusQuerier(AbstractMetricQuerier):
    prometheus_url = "https://prometheus.d8-monitoring:9090/api/v1/query"

    def __init__(self, token: str):
        super().__init__()
        self.token = token

    def query(self, query: str) -> List[Dict[str, Any]]:
        """query prometheus from query and get api result response"""

        response = self.do_get_request(
            url=self.prometheus_url,
            params={"query": query},
            headers={"Authorization": f"Bearer {self.token}"},
            decode_json=True,
        )
        # response:
        # {"status": "success", "data": {"resultType": "vector", "result": [{"metric": {"__name__": "name"}, "value": ["timestamp", "value"]}]}}
        if response["status"] != "success":
            raise ResponseError(f"error quering prometheus with query '{query}'")

        return response.get("data", {}).get("result", [0])

    def query_value(self, query: str) -> float:
        """query prometheus from query and get only value result"""

        query_result = self.query(query)
        if len(query_result) > 0:
            return float(query_result[0].get("value", [0, 0])[1])
        return 0.0
