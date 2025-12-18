# [<en>] Section containing the parameters of Ingress NGINX Controller.
# [<en>] https://deckhouse.io/modules/ingress-nginx/cr.html
# [<ru>] Секция, описывающая параметры Ingress NGINX Controller.
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
