#!/usr/bin/env python3

import kubernetes
import collections
from http.server import BaseHTTPRequestHandler, HTTPServer

kubernetes.config.load_incluster_config()

corev1 = kubernetes.client.CoreV1Api()
appsv1 = kubernetes.client.AppsV1Api()
batchv1 = kubernetes.client.BatchV1Api()
extensionsv1beta1 = kubernetes.client.ExtensionsV1beta1Api()
custom_api = kubernetes.client.CustomObjectsApi()


def dict_merge(dct, merge_dct):
    """ Recursive dict merge. Inspired by :meth:``dict.update()``, instead of
    updating only top-level keys, dict_merge recurses down into dicts nested
    to an arbitrary depth, updating keys. The ``merge_dct`` is merged into
    ``dct``.
    :param dct: dict onto which the merge is executed
    :param merge_dct: dct merged into dct
    :return: None
    """
    for k, v in merge_dct.items():
        if (k in dct and isinstance(dct[k], dict)
                and isinstance(merge_dct[k], collections.Mapping)):
            dict_merge(dct[k], merge_dct[k])
        else:
            dct[k] = merge_dct[k]


def tranform_to_dict(object_list):
    to_return = {}
    for object in object_list.items:
        dict_merge(to_return, {object.metadata.namespace: {object.metadata.name: object}})

    return to_return


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
        vpas = []
        for namespace in corev1.list_namespace().items:
            for vpa in \
                    custom_api.list_namespaced_custom_object("autoscaling.k8s.io", "v1beta2", namespace.metadata.name,
                                                             "verticalpodautoscalers")["items"]:
                vpas.append(vpa)

        response = """# HELP vpa_recommendation Per-container VPA recommendations
# TYPE vpa_recommendation gauge\n"""

        for vpa in vpas:
            try:
                vpa_target_ref = vpa["spec"]["targetRef"]
                vpa_container_recommendations = vpa["status"]["recommendation"]["containerRecommendations"]
            except KeyError as e:
                print('One of the required fields on a VPA object {}/{} does not exist: {}'.format(
                    vpa["metadata"]["namespace"], vpa["metadata"]["name"], e))
                continue

            for container in vpa_container_recommendations:
                container_name = container["containerName"]
                for recommendation_type, recommendation_value in container.items():
                    if recommendation_type != "containerName":
                        for resource_type, resource_value in recommendation_value.items():
                            response += 'vpa_recommendation{{namespace="{}", vpa="{}", update_policy="{}", controller_name="{}", controller_type="{}", container="{}", recommendation_type="{}", resource_type="{}"}} {}\n'.format(
                                vpa["metadata"]["namespace"], vpa["metadata"]["name"],
                                vpa["spec"]["updatePolicy"]["updateMode"], vpa_target_ref["name"], vpa_target_ref["kind"],
                                container_name,
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
