---
title: How to redirect traffic to a virtual machine?
sections:
- vm_configuration
lang: en
---

The virtual machine runs in a Kubernetes cluster, so directing network traffic to it works like routing traffic to pods. To route traffic to a virtual machine, use the standard Kubernetes mechanism — the Service resource, which selects targets using a label selector.

1. Create a service with the required settings.

   For example, consider a virtual machine with the label `vm: frontend-0`, an HTTP service exposed on ports 80 and 443, and SSH access on port 22:

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualMachine
   metadata:
     name: frontend-0
     namespace: dev
     labels:
       vm: frontend-0
   spec: ...
   ```

1. To route network traffic to the virtual machine's ports, create the following Service:

1. To route network traffic to the virtual machine's ports, create the following service:

This service listens on ports 80 and 443 and forwards traffic to the target virtual machine’s ports 80 and 443. SSH access from outside the cluster is provided on port 2211.

   ```yaml
   apiVersion: v1
   kind: Service
   metadata:
     name: frontend-0-svc
     namespace: dev
   spec:
     type: LoadBalancer
     ports:
     - name: ssh
       port: 2211
       protocol: TCP
       targetPort: 22
     - name: http
       port: 80
       protocol: TCP
       targetPort: 80
     - name: https
       port: 443
       protocol: TCP
       targetPort: 443
     selector:
       vm: frontend-0
   ```
