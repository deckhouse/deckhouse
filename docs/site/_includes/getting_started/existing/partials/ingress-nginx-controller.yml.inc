# [<en>] section containing the parameters of nginx ingress controller
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры nginx ingress controller
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  # [<en>] the name of the Ingress class to use with the Ingress nginx controller
  # [<ru>] имя Ingress-класса для обслуживания Ingress NGINX controller
  ingressClass: nginx
  # [<en>] the way traffic goes to cluster from the outer network
  # [<ru>] способ поступления трафика из внешнего мира
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
  # [<en>] describes on which nodes the component will be located
  # [<ru>] описывает, на каких узлах будет находиться компонент
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  nodeSelector:
    node-role.kubernetes.io/master: ""
  tolerations:
  - operator: Exists
