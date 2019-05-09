#!/usr/bin/env python3

import kubernetes
import copy
from abc import ABC, abstractmethod
from http.server import BaseHTTPRequestHandler, HTTPServer

kubernetes.config.load_incluster_config()

EXTENDED_MONITORING_ANNOTATION_THRESHOLD_PREFIX = "threshold.extended-monitoring.flant.com/"
EXTENDED_MONITORING_ENABLED_ANNOTATION = "extended-monitoring.flant.com/enabled"


class Annotated(ABC):
    default_thresholds = {}

    def __init__(self, namespace, name, kube_annotations):
        self.namespace = namespace
        self.name = name
        self.enabled = True

        if kube_annotations:
            if not {EXTENDED_MONITORING_ENABLED_ANNOTATION: "false"}.items() <= kube_annotations.items():
                self.thresholds = copy.deepcopy(self.default_thresholds)
                for name, value in kube_annotations.items():
                    if name.startswith(EXTENDED_MONITORING_ANNOTATION_THRESHOLD_PREFIX):
                        self.thresholds.update(
                            {name.replace(EXTENDED_MONITORING_ANNOTATION_THRESHOLD_PREFIX, ""): value})
            else:
                self.enabled = False

    @classmethod
    def list_threshold_annotated_objects(cls, namespace):
        exported = []
        for kube_object in cls.list(namespace):
            exported.append(cls(namespace, kube_object.metadata.name, kube_object.metadata.annotations))

        return exported

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
    def list(cls, namespace):
        pass


# TODO: Stuck Apps in Controller
class AnnotatedDeployment(Annotated):
    kind = "Deployment"
    api = kubernetes.client.AppsV1Api()

    @classmethod
    def list(cls, namespace):
        return cls.api.list_namespaced_deployment(namespace).items

    default_thresholds = {
        "replicas-not-ready": 0
    }


class AnnotatedStatefulSet(Annotated):
    kind = "StatefulSet"
    api = kubernetes.client.AppsV1Api()

    @classmethod
    def list(cls, namespace):
        return cls.api.list_namespaced_stateful_set(namespace).items

    default_thresholds = {
        "replicas-not-ready": 0
    }


class AnnotatedDaemonSet(Annotated):
    kind = "DaemonSet"
    api = kubernetes.client.AppsV1Api()

    @classmethod
    def list(cls, namespace):
        return cls.api.list_namespaced_daemon_set(namespace).items

    default_thresholds = {
        "replicas-not-ready": 0
    }


class AnnotatedPod(Annotated):
    kind = "Pod"
    api = kubernetes.client.CoreV1Api()

    @classmethod
    def list(cls, namespace):
        return cls.api.list_namespaced_pod(namespace).items

    default_thresholds = {
        "disk-bytes-warning": 70,
        "disk-bytes-critical": 80,
        "disk-inodes-warning": 85,
        "disk-inodes-critical": 90,
        "container-throttling-warning": 25,
        "container-throttling-critical": 50
    }


class AnnotatedIngress(Annotated):
    kind = "Ingress"
    api = kubernetes.client.ExtensionsV1beta1Api()

    @classmethod
    def list(cls, namespace):
        return cls.api.list_namespaced_ingress(namespace).items

    default_thresholds = {
        "5xx-warning": 10,
        "5xx-critical": 20
    }


class AnnotatedNode(Annotated):
    kind = "Node"
    api = kubernetes.client.CoreV1Api()

    @classmethod
    def list(cls, namespace=None):
        return cls.api.list_node().items

    default_thresholds = {
        "disk-bytes-warning": 70,
        "disk-bytes-critical": 80,
        "disk-inodes-warning": 85,
        "disk-inodes-critical": 90
    }


class AnnotatedCronJob(Annotated):
    kind = "CronJob"
    api = kubernetes.client.BatchV1beta1Api()

    @classmethod
    def list(cls, namespace):
        return cls.api.list_namespaced_cron_job(namespace).items


KUBERNETES_OBJECTS = (AnnotatedNode,)
KUBERNETES_NAMESPACED_OBJECTS = (
    AnnotatedDeployment, AnnotatedStatefulSet, AnnotatedDaemonSet, AnnotatedPod, AnnotatedIngress, AnnotatedCronJob)

corev1 = kubernetes.client.CoreV1Api()
apis = kubernetes.client.ApisApi()

class GetHandler(BaseHTTPRequestHandler):

    def do_GET(self):
        if self.path == "/ready":
            api_response = apis.get_api_versions()
            self.send_response(200)
            self.end_headers()
        elif self.path == "/healthz":
            self.send_response(200)
            self.end_headers()
        else:
            exported = []

            # iterate over namespaced objects in explicitly enabled via annotation Namespaces
            ns_list = corev1.list_namespace()
            for namespace in (ns for ns in ns_list.items if ns.metadata.annotations and
                                                            EXTENDED_MONITORING_ENABLED_ANNOTATION in ns.metadata.annotations.keys()):
                for kube_object in KUBERNETES_NAMESPACED_OBJECTS:
                    exported.extend(kube_object.list_threshold_annotated_objects(namespace.metadata.name))

            for kube_object in KUBERNETES_OBJECTS:
                exported.extend(kube_object.list_threshold_annotated_objects(None))

            response = """# HELP extended_monitoring_annotations Extended monitoring annotations
            # TYPE extended_monitoring_annotations gauge\n"""
            for annotated_object in exported:
                response += annotated_object.formatted

            self.send_response(200)
            self.send_header('Content-Type',
                             'text/plain; charset=utf-8')
            self.end_headers()
            self.wfile.write(response.encode(encoding="utf-8"))


if __name__ == '__main__':
    server = HTTPServer(('0.0.0.0', 8080), GetHandler)
    print('Starting server...')
    server.serve_forever()
