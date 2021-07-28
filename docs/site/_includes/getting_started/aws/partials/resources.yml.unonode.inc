# [<en>] section containing the parameters of nginx ingress controller
# [<ru>] секция, описывающая параметры nginx ingress controller
# [<en>] version of the Deckhouse API
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  # [<en>] the name of the Ingress class to use with the Ingress nginx controller
  # [<ru>] имя Ingress-класса для использования Ingress Nginx controller
  ingressClass: nginx
  # [<en>] the way traffic goes to cluster from the outer network
  # [<ru>] способ поступления трафика из внешнего мира
  inlet: LoadBalancer
  loadBalancer:
    # [<en>] annotations to assign to the service for flexible configuration of the load balancer
    # [<ru>] аннотации, которые будут проставлены сервису для гибкой настройки балансировщика
    annotations:
      # [<en>] provision Network Load Balancer for the Ingress Controller Service
      # [<ru>] использовать Network Load Balancer для сервиса Ingress контроллера
      service.beta.kubernetes.io/aws-load-balancer-type: nlb
  # [<en>] describes on which nodes the component will be located
  # [<ru>] описывает, на каких узлах будет находиться компонент
  nodeSelector:
    node-role.kubernetes.io/master: ""
---
# [<en>] section containing the parameters of the static user
# [<ru>] секция, описывающая параметры статического пользователя
# [<en>] version of the Deckhouse API
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  # [<en>] user e-mail
  # [<ru>] e-mail пользователя
  email: admin@example.com
  # [<en>] this is a hash for generated password: <GENERATED_PASSWORD>
  # [<ru>] это хэш сгенерированного пароля: <GENERATED_PASSWORD>
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  password: <GENERATED_PASSWORD_HASH>
