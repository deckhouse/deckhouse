# [<en>] Section containing the parameters of NGINX Ingress controller.
# [<en>] https://deckhouse.io/modules/ingress-nginx/cr.html
# [<ru>] Секция, описывающая параметры NGINX Ingress controller.
# [<ru>] https://deckhouse.ru/modules/ingress-nginx/cr.html
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  # [<en>] The way traffic goes to cluster from the outer network.
  # [<ru>] Способ поступления трафика из внешнего мира.
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
  # [<en>] Describes on which nodes the Ingress Controller will be located.
  # [<ru>] Описывает, на каких узлах будет находиться Ingress-контроллер.
  # [<en>] You might consider changing this.
  # [<en>] More examples here.
  # [<en>] https://deckhouse.io/modules/ingress-nginx/examples.html
  # [<ru>] Возможно, захотите изменить.
  # [<ru>] Больше примеров здесь
  # [<ru>] https://deckhouse.ru/modules/ingress-nginx/examples.html
  nodeSelector:
    node-role.kubernetes.io/control-plane: ""
  tolerations:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
    operator: Exists
