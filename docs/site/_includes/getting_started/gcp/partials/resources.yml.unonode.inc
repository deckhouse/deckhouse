---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  # [<en>] Ingress version to use (use version 1.1 with Kubernetes 1.23+)
  # [<ru>] версия Ingress-контроллера (используйте версию 1.1 с Kubernetes 1.23+)
  controllerVersion: "1.1"
  tolerations:
  - key: node-role.kubernetes.io/master
  - key: node-role.kubernetes.io/control-plane
