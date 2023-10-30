#!/usr/bin/env python3

# Copyright 2021 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from concurrent.futures.thread import ThreadPoolExecutor
from datetime import datetime, timedelta
from itertools import chain
from threading import Thread
from time import sleep
import logging

import kubernetes
import copy
import sys

from abc import ABC, abstractmethod
from http.server import BaseHTTPRequestHandler, HTTPServer
from socketserver import ThreadingMixIn

kubernetes.config.load_incluster_config()

logging.basicConfig(format='[%(asctime)s] - %(message)s', level=logging.INFO)

DEPRECATED_EXTENDED_MONITORING_ANNOTATION_THRESHOLD_PREFIX = "threshold.extended-monitoring.flant.com/"
DEPRECATED_EXTENDED_MONITORING_ENABLED_ANNOTATION = "extended-monitoring.flant.com/enabled"
EXTENDED_MONITORING_LABEL_THRESHOLD_PREFIX = "threshold.extended-monitoring.deckhouse.io/"
EXTENDED_MONITORING_ENABLED_LABEL = "extended-monitoring.deckhouse.io/enabled"

DEFAULT_SERVER_ADDRESS = '0.0.0.0'
DEFAULT_PORT = 8080


class ThreadingHTTPServer(ThreadingMixIn, HTTPServer):
    daemon_threads = True


# Monitoring is enabled by default for all controllers in a namespace and can be disabled by using
# the 'extended-monitoring.deckhouse.io/enabled=false' label
# or the 'extended-monitoring.flant.com/enabled=false' annotation.
def is_monitoring_enabled(labels, annotations):
    if annotations and annotations.get(DEPRECATED_EXTENDED_MONITORING_ENABLED_ANNOTATION, "true") == "false":
        return False
    if labels and labels.get(EXTENDED_MONITORING_ENABLED_LABEL, "true") == "false":
        return False
    return True


def parse_thresholds(labels, annotations, default_thresholds):
    thresholds = copy.deepcopy(default_thresholds)
    if annotations:
        for name, value in annotations.items():
            if name.startswith(DEPRECATED_EXTENDED_MONITORING_ANNOTATION_THRESHOLD_PREFIX):
                unprefixed_name = name.replace(DEPRECATED_EXTENDED_MONITORING_ANNOTATION_THRESHOLD_PREFIX, "")
                thresholds[unprefixed_name] = value
    if labels:
        for name, value in labels.items():
            if name.startswith(EXTENDED_MONITORING_LABEL_THRESHOLD_PREFIX):
                unprefixed_name = name.replace(EXTENDED_MONITORING_LABEL_THRESHOLD_PREFIX, "")
                thresholds[unprefixed_name] = value

    return thresholds


class Annotated(ABC):
    default_thresholds = {}
    # When specified with a watch call, shows changes that occur after that particular version of a resource.
    # Defaults to changes from the beginning of history. When specified for list:
    # - if unset, then the result is returned from remote storage based on quorum-read flag;
    # - if it's 0, then we simply return what we currently have in cache, no guarantee;
    # - if set to non zero, then the result is at least as fresh as given rv.
    default_list_options = {
        "resource_version": 0,
    }

    def __init__(self, namespace, name, kube_labels, kube_annotations):
        self.namespace = namespace
        self.name = name
        self.enabled = is_monitoring_enabled(kube_labels, kube_annotations)
        self.thresholds = parse_thresholds(kube_labels, kube_annotations, self.default_thresholds)

    @classmethod
    def list_threshold_annotated_objects(cls, namespace):
        for kube_object in cls.list(namespace, **cls.default_list_options):
            yield cls(namespace, kube_object.metadata.name, kube_object.metadata.labels,
                      kube_object.metadata.annotations)

    @property
    def formatted(self):
        to_return = ""

        if self.enabled:
            to_return += 'extended_monitoring_{}_enabled{{namespace="{}", {}="{}"}} {}\n'.format(
                self.kind.lower(),
                self.namespace,
                self.kind.lower(),
                self.name, 1)

            if hasattr(self, "thresholds"):
                for k, v in self.thresholds.items():
                    to_return += 'extended_monitoring_{}_threshold{{namespace="{}", threshold="{}", {}="{}"}} {}\n'.format(
                        self.kind.lower(),
                        self.namespace,
                        k,
                        self.kind.lower(),
                        self.name, int(v))
        else:
            to_return += 'extended_monitoring_{}_enabled{{namespace="{}", {}="{}"}} {}\n'.format(
                self.kind.lower(),
                self.namespace,
                self.kind.lower(),
                self.name, 0)

        return to_return

    @property
    @abstractmethod
    def kind(self):
        pass

    @property
    @abstractmethod
    def api(self):
        pass

    @classmethod
    @abstractmethod
    def list(cls, namespace, **kwargs):
        pass


class AnnotatedDeployment(Annotated):
    kind = "Deployment"
    api = kubernetes.client.AppsV1Api()

    @classmethod
    def list(cls, namespace, **kwargs):
        return cls.api.list_namespaced_deployment(namespace, **kwargs).items

    default_thresholds = {
        "replicas-not-ready": 0
    }


class AnnotatedStatefulSet(Annotated):
    kind = "StatefulSet"
    api = kubernetes.client.AppsV1Api()

    @classmethod
    def list(cls, namespace, **kwargs):
        return cls.api.list_namespaced_stateful_set(namespace, **kwargs).items

    default_thresholds = {
        "replicas-not-ready": 0
    }


class AnnotatedDaemonSet(Annotated):
    kind = "DaemonSet"
    api = kubernetes.client.AppsV1Api()

    @classmethod
    def list(cls, namespace, **kwargs):
        return cls.api.list_namespaced_daemon_set(namespace, **kwargs).items

    default_thresholds = {
        "replicas-not-ready": 0
    }


class AnnotatedPod(Annotated):
    kind = "Pod"
    api = kubernetes.client.CoreV1Api()

    @classmethod
    def list(cls, namespace, **kwargs):
        return cls.api.list_namespaced_pod(namespace, **kwargs).items

    default_thresholds = {
        "disk-bytes-warning": 85,
        "disk-bytes-critical": 95,
        "disk-inodes-warning": 85,
        "disk-inodes-critical": 90,
        "container-throttling-warning": 25,
        "container-throttling-critical": 50,
    }


class AnnotatedIngress(Annotated):
    kind = "Ingress"
    api = kubernetes.client.NetworkingV1Api()

    @classmethod
    def list(cls, namespace, **kwargs):
        return cls.api.list_namespaced_ingress(namespace, **kwargs).items

    default_thresholds = {
        "5xx-warning": 10,
        "5xx-critical": 20
    }


class AnnotatedNode(Annotated):
    kind = "Node"
    api = kubernetes.client.CoreV1Api()

    @classmethod
    def list(cls, namespace=None, **kwargs):
        return cls.api.list_node(**kwargs).items

    default_thresholds = {
        "disk-bytes-warning": 70,
        "disk-bytes-critical": 80,
        "disk-inodes-warning": 90,
        "disk-inodes-critical": 95,
        "load-average-per-core-warning": 3,
        "load-average-per-core-critical": 10,
    }


class AnnotatedCronJob(Annotated):
    kind = "CronJob"
    api = kubernetes.client.BatchV1Api()

    @classmethod
    def list(cls, namespace, **kwargs):
        return cls.api.list_namespaced_cron_job(namespace, **kwargs).items


KUBERNETES_OBJECTS = (
    AnnotatedNode,
)
KUBERNETES_NAMESPACED_OBJECTS = (
    AnnotatedDeployment,
    AnnotatedStatefulSet,
    AnnotatedDaemonSet,
    AnnotatedPod,
    AnnotatedIngress,
    AnnotatedCronJob,
)

corev1 = kubernetes.client.CoreV1Api()
apis = kubernetes.client.ApisApi()


def _list_objects(executor, objects, namespace):
    yield from chain.from_iterable(executor.map(lambda k: k.list_threshold_annotated_objects(namespace), objects))


def _get_metrics():
    enabled_nses = []
    quantity = 0

    # iterate over namespaced objects in explicitly enabled via annotation Namespaces
    ns_list = (
        ns.metadata.name for ns in corev1.list_namespace().items
        if
        (ns.metadata.labels and EXTENDED_MONITORING_ENABLED_LABEL in ns.metadata.labels)
        or
        (ns.metadata.annotations and DEPRECATED_EXTENDED_MONITORING_ENABLED_ANNOTATION in ns.metadata.annotations)
    )

    response = """# HELP extended_monitoring_annotations Extended monitoring annotations
      # TYPE extended_monitoring_annotations gauge\n"""

    with ThreadPoolExecutor(max_workers=8) as executor:
        def _list_in_ns(ns):
            enabled_nses.append('\nextended_monitoring_enabled{{namespace="{}"}} 1'.format(ns))
            yield from _list_objects(executor, KUBERNETES_NAMESPACED_OBJECTS, ns)

        for annotated_object in chain.from_iterable(executor.map(_list_in_ns, ns_list)):
            response += annotated_object.formatted
            quantity += 1

        for annotated_object in _list_objects(executor, KUBERNETES_OBJECTS, None):
            response += annotated_object.formatted
            quantity += 1

    response += '\n'.join(enabled_nses)
    quantity += len(enabled_nses)
    return response, quantity


class GetHandler(BaseHTTPRequestHandler):
    _response = ""
    _populated = False
    _last_observe = datetime.now()

    @classmethod
    def get_metrics(cls):
        # setting string variable is atomic in Python
        cls._response, quantity = _get_metrics()
        logging.info('Metrics are collected successfully. Batches quantity: {}'.format(quantity))
        cls._last_observe = datetime.now()

    @classmethod
    def loop_get_metrics(cls):
        cls.get_metrics()
        cls._populated = True
        sleep(30)

        while 1:
            try:
                cls.get_metrics()
            except Exception as loop_err:
                logging.info(str(loop_err))
            sleep(30)

    def do_GET(self):
        if self.path == "/ready":
            apis.get_api_versions()
            # Wait for the first metrics request to succeed
            self.send_response(200 if self.__class__._populated else 400)
            self.end_headers()
            return

        if self.path == "/healthz":
            # Fail if metrics were last collected more than 15 minutes ago
            fresh_enough = self.__class__._last_observe > (datetime.now() - timedelta(minutes=15))
            self.send_response(200 if fresh_enough else 500)
            self.end_headers()
            logging.info('Last observed time: {}'.format(self.__class__._last_observe.strftime("%m/%d/%Y, %H:%M:%S")))
            return

        if self.path == "/metrics":
            self.send_response(200)
            self.send_header('Content-Type',
                             'text/plain; charset=utf-8')
            self.end_headers()
            self.wfile.write(self.__class__._response.encode(encoding="utf-8"))
            return

        self.send_response(404)
        self.end_headers()


if __name__ == '__main__':
    server_address = DEFAULT_SERVER_ADDRESS
    server_port = DEFAULT_PORT

    # Parse host and port
    if len(sys.argv) >= 2:
        server_address = sys.argv[1]
    if len(sys.argv) == 3:
        server_port = int(sys.argv[2])

    server = ThreadingHTTPServer((server_address, server_port), GetHandler)

    try:
        # Run metrics renew in background (daemon thread is canceled on the script exit)
        logging.info('Starting metrics loop')
        Thread(target=GetHandler.loop_get_metrics, daemon=True).start()

        logging.info('Starting server')
        server.serve_forever()
    except Exception as err:
        logging.info('Shutting down server')
        raise err
