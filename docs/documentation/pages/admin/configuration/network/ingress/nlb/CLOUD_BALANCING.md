---
title: "Balancing in clusters on cloud platforms"
permalink: en/admin/configuration/network/ingress/nlb/cloud-balancing.html
description: "Configure load balancing in cloud clusters for Deckhouse Kubernetes Platform. AWS, GCP, Azure load balancer integration and cloud-native load balancing setup."
---

A list of supported providers:

* [Amazon Web Services](https://aws.amazon.com/)
* [Google Cloud Platform](https://cloud.google.com/)
* [Microsoft Azure](https://azure.microsoft.com/)
* [OpenStack](https://www.openstack.org/)
* [Huawei Cloud](https://cloud.huawei.com/)
* [VMware Cloud DirectorExperimental](https://www.vmware.com/products/cloud-infrastructure/cloud-director)
* [VMware vSphere](https://www.vmware.com/products/cloud-infrastructure/vsphere)
* [Yandex Cloud](https://yandex.cloud/)
* zVirtExperimental

Configuring incoming traffic balancing in clusters on cloud platforms
involves creating an Ingress controller with specified LoadBalancer parameters.
Based on these settings, the cloud provider automatically creates an external load balancer.
In the cluster, a Service resource is created through which the external load balancer routes traffic to applications.

## Example of creating an Ingress controller for the OpenStack provider

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancerWithProxyProtocol
  loadBalancerWithProxyProtocol:
    annotations:
      loadbalancer.openstack.org/proxy-protocol: "true"
      loadbalancer.openstack.org/timeout-member-connect: "2000"
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    operator: Equal
    value: frontend
```

## Example of creating a ClusterIP-type Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: backend-resolver-cluster-ip
spec:
  ports:
  - name: http
    port: 8000
    protocol: TCP
  selector:
    app: lab-4-backend
  type: ClusterIP
```

### Example for VK Cloud

The following example is relevant when the internal balancer would be used.

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  loadBalancer:
    annotations:
      service.beta.kubernetes.io/openstack-internal-load-balancer: "true"
  nodeSelector:
    node.deckhouse.io/group: worker
```
