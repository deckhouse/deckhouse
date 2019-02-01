#!/usr/bin/env python3

import kubernetes
import copy
from http.server import BaseHTTPRequestHandler, HTTPServer

kubernetes.config.load_incluster_config()

corev1 = kubernetes.client.CoreV1Api()
custom_api = kubernetes.client.CustomObjectsApi()


def gather():
    pods_by_ns = {}
    vpas = []

    pods = corev1.list_pod_for_all_namespaces().items
    for pod in pods:
        pods_by_ns.setdefault(pod.metadata.namespace, []).append(pod)

    for namespace in corev1.list_namespace().items:
        for vpa in custom_api.list_namespaced_custom_object("autoscaling.k8s.io", "v1beta1", namespace.metadata.name,
                                                            "verticalpodautoscalers")["items"]:
            vpas.append(vpa)

    return pods_by_ns, vpas


def convert_cpu(cpu: str):
    if cpu.endswith("m"):
        stripped = cpu.rstrip("m")
        return int(stripped) / 1000
    return cpu


class GetHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        pods, vpas = gather()

        response = """# HELP vpa_recommendation Per-container VPA recommendations
# TYPE vpa_recommendation gauge\n"""

        for vpa in vpas:
            vpa_label_selector = vpa["spec"]["selector"]["matchLabels"]
            pods_in_ns = (pod_ns for pod_ns in pods[vpa["metadata"]["namespace"]])
            matching_pods = (matching_pod for matching_pod in pods_in_ns if
                             matching_pod.metadata.labels.items() >= vpa_label_selector.items())
            for pod in matching_pods:
                for container in vpa["status"]["recommendation"]["containerRecommendations"]:
                    container_name = container["containerName"]
                    for recommendation_type, recommendation_value in container.items():
                        if recommendation_type != "containerName":
                            for resource_type, resource_value in recommendation_value.items():
                                response += 'vpa_recommendation{{namespace="{}", pod="{}", container="{}", recommendation_type="{}", resource_type ="{}"}} {}\n'.format(
                                    vpa["metadata"]["namespace"], pod.metadata.name, container_name,
                                    recommendation_type, resource_type, convert_cpu(resource_value))

        self.send_response(200)
        self.send_header('Content-Type',
                         'text/plain; charset=utf-8')
        self.end_headers()
        self.wfile.write(response.encode(encoding="utf-8"))


if __name__ == '__main__':
    server = HTTPServer(('0.0.0.0', 8080), GetHandler)
    print('Starting server...')
    server.serve_forever()
