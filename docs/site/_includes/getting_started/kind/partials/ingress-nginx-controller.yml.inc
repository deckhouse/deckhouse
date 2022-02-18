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
  # [<en>] The way traffic goes to cluster from the outer network.
  # [<ru>] Способ поступления трафика из внешнего мира.
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
