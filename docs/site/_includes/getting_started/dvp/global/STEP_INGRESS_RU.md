Убедитесь, что под Kruise controller manager модуля [ingress-nginx](https://TODO) запустился и находится в статусе `Ready`.
  Выполните на **master-узле** следующую команду:

{% snippetcut %}
```shell
sudo d8 k -n d8-ingress-nginx get po -l app=kruise
```
{% endsnippetcut %}

Создайте Ingress-контроллер:

{% snippetcut %}
```shell
sudo d8 k apply -f - <<EOF
# Секция, описывающая параметры NGINX Ingress controller.
# https://deckhouse.ru/products/virtualization-platform/reference/cr/ingressnginxcontroller.html
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  # Способ поступления трафика из внешнего мира.
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
  # Описывает, на каких узлах будет находиться Ingress-контроллер.
  # Возможно, захотите изменить.
  # Больше примеров здесь
  # https://TODO
  nodeSelector:
    node-role.kubernetes.io/control-plane: ""
  tolerations:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
    operator: Exists
EOF
```
{% endsnippetcut %}

Запуск Ingress-контроллера после завершения установки Deckhouse может занять какое-то время. Прежде чем продолжить убедитесь что Ingress-контроллер запустился (выполните на `master-узле`):


{% snippetcut %}
```shell
sudo d8 k -n d8-ingress-nginx get po -l app=controller
```
{% endsnippetcut %}

Дождитесь перехода подов Ingress-контроллера в статус `Ready`.

{% offtopic title="Пример вывода..." %}
```
$ sudo /opt/deckhouse/bin/kubectl -n d8-ingress-nginx get po -l app=controller
NAME                                       READY   STATUS    RESTARTS   AGE
controller-nginx-r6hxc                     3/3     Running   0          5m
```
{%- endofftopic %}
