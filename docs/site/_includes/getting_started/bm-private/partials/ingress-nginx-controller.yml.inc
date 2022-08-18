# [<en>] Section containing the parameters of nginx ingress controller.
# [<en>] Version of the Deckhouse API.
# [<ru>] Секция, описывающая параметры nginx ingress controller.
# [<ru>] Используемая версия API Deckhouse.
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  # [<en>] The name of the Ingress class to use with the Ingress nginx controller.
  # [<ru>] Имя Ingress-класса для обслуживания Ingress NGINX controller.
  ingressClass: nginx
  # [<en>] Ingress version to use (use version 1.1 with Kubernetes 1.23+)
  # [<ru>] версия Ingress-контроллера (используйте версию 1.1 с Kubernetes 1.23+)
  controllerVersion: "1.1"
  # [<en>] The way traffic goes to cluster from the outer network.
  # [<ru>] Способ поступления трафика из внешнего мира.
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
  # [<en>] Describes on which nodes the component will be located.
  # [<ru>] Описывает, на каких узлах будет находиться компонент.
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  nodeSelector:
    node-role.kubernetes.io/master: ""
  tolerations:
  - operator: Exists
