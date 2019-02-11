#!/usr/bin/env python3

import kubernetes
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


def convert(resource: str):
    unit = {
        "Ki": 2 ** 10,
        "Mi": 2 ** 20,
        "Gi": 2 ** 30,
        "Ti": 2 ** 40,
        "Pi": 2 ** 50,
        "Ei": 2 ** 60,
        "n": 0.000000001,
        "u": 0.000001,
        "m": 0.001,
        "k": 10 ** 3,
        "M": 10 ** 6,
        "G": 10 ** 9,
        "T": 10 ** 12,
        "P": 10 ** 15,
        "E": 10 ** 18,

    }
    return int(''.join(c for c in resource if c.isdigit())) * unit.get(''.join(c for c in resource if c.isalpha()), 1)


class GetHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        pods, vpas = gather()

        response = """# HELP vpa_recommendation Per-container VPA recommendations
# TYPE vpa_recommendation gauge\n"""

        for vpa in vpas:
            try:
                vpa_label_selector = vpa["spec"]["selector"]["matchLabels"]
                vpa_container_recommendations = vpa["status"]["recommendation"]["containerRecommendations"]
            except KeyError as e:
                print('One of required fields on a VPA object {}/{} does not exist: {}'.format(
                    vpa["metadata"]["namespace"], vpa["metadata"]["name"], e))
                continue
            pods_in_ns = (pod_ns for pod_ns in pods[vpa["metadata"]["namespace"]])
            matching_pods = []
            for pod in pods_in_ns:
                try:
                    if pod.metadata.labels.items() >= vpa_label_selector.items():
                        matching_pods.append(pod)
                except AttributeError:
                    print('Pod "{}" has no labels, skipping'.format(pod.metadata.name))

            for pod in matching_pods:
                for container in vpa_container_recommendations:
                    container_name = container["containerName"]
                    for recommendation_type, recommendation_value in container.items():
                        if recommendation_type != "containerName":
                            for resource_type, resource_value in recommendation_value.items():
                                response += 'vpa_recommendation{{namespace="{}", vpa="{}", update_policy="{}", pod="{}", container="{}", recommendation_type="{}", resource_type ="{}"}} {}\n'.format(
                                    vpa["metadata"]["namespace"], vpa["metadata"]["name"],
                                    vpa["spec"]["updatePolicy"]["updateMode"], pod.metadata.name, container_name,
                                    recommendation_type, resource_type, convert(resource_value))

        self.send_response(200)
        self.send_header('Content-Type',
                         'text/plain; charset=utf-8')
        self.end_headers()
        self.wfile.write(response.encode(encoding="utf-8"))


if __name__ == '__main__':
    server = HTTPServer(('0.0.0.0', 8080), GetHandler)
    print('Starting server...')
    server.serve_forever()
