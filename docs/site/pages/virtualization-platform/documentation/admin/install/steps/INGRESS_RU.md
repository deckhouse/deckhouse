---
title: "Настройка Ingress и DNS"
permalink: ru/virtualization-platform/documentation/admin/install/steps/ingress.html
lang: ru
---

## Настройка Ingress

Убедитесь, что под Kruise controller manager модуля [ingress-nginx](https://TODO) запустился и находится в статусе `Ready`.
Выполните на **master-узле** следующую команду:

  ```shell
  sudo -i d8 k -n d8-ingress-nginx get po -l app=kruise
  ```

Создайте ресурс IngressNginxController, описывающий параметры NGINX Ingress controller.

```shell
sudo -i d8 k apply -f - <<EOF
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
  nodeSelector:
    node-role.kubernetes.io/control-plane: ""
  tolerations:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
    operator: Exists
EOF
```

Запуск Ingress-контроллера может занять какое-то время. Убедитесь что поды Ingress-контроллера перешли в статус  `Running`, выполнив команду:

```shell
sudo -i d8 k -n d8-ingress-nginx get po -l app=controller
```

{% offtopic title="Пример вывода..." %}
```
NAME                                       READY   STATUS    RESTARTS   AGE
controller-nginx-r6hxc                     3/3     Running   0          5m
```
{% endofftopic %}


## Настройка DNS

Для доступа к веб-интерфейсам платформы необходимо настроить DNS-записи для домена кластера.

**Важно:** Домен, используемый в шаблоне, не должен совпадать с доменом, указанным в параметре clusterDomain и внутренней сервисной зоне сети. Например, если используется `clusterDomain: cluster.local` (значение по умолчанию), а сервисная зона сети — `ru-central1.internal`, то `publicDomainTemplate` не может быть `%s.cluster.local` или `%s.ru-central1.internal`.

### Wildcard-домен

Поддомены должны резолвиться в IP адрес узла, где запускается nginx-controller. В нашем случае это master-0. Так же убедитесь, что шаблон имён имеет вид %s.<домен>:

```shell
sudo -i d8 k get mc global -ojson | jq -r '.spec.settings.modules.publicDomainTemplate'
```

Пример вывода, если использовался свой wildcard-домен:
```
%s.my-dvp-cluster.example.com
```

Пример вывода, если использовался домен от сервиса ssslip.io:

```
%s.54.43.32.21.sslip.io
```

### Не wildcard-домен

Если в шаблоне указан не wildcard-домен, то нужно создать дополнительные А или CNAME-записи со значением публичного IP-адреса, где запускается nginx-controller. Записи нужны для всех сервисов Deckhouse.

Например, для домена my-dvp-cluster.example.com и шаблона с поддоменами %s.my-dvp-cluster.example.com, записи будут выглядеть так:

```
api.my-dvp-cluster.example.com
argocd.my-dvp-cluster.example.com
dashboard.my-dvp-cluster.example.com
documentation.my-dvp-cluster.example.com
dex.my-dvp-cluster.example.com
grafana.my-dvp-cluster.example.com
hubble.my-dvp-cluster.example.com
istio.my-dvp-cluster.example.com
istio-api-proxy.my-dvp-cluster.example.com
kubeconfig.my-dvp-cluster.example.com
openvpn-admin.my-dvp-cluster.example.com
prometheus.my-dvp-cluster.example.com
status.my-dvp-cluster.example.com
upmeter.my-dvp-cluster.example.com
```

Для домена my-dvp-cluster.example.com и шаблона с индивидуальными доменами `%s-my-dvp-cluster.example.com`, записи будут выглядеть так:

```
api-my-dvp-cluster.example.com
argocd-my-dvp-cluster.example.com
dashboard-my-dvp-cluster.example.com
documentation-my-dvp-cluster.example.com
dex-my-dvp-cluster.example.com
grafana-my-dvp-cluster.example.com
hubble-my-dvp-cluster.example.com
istio-my-dvp-cluster.example.com
istio-api-proxy-my-dvp-cluster.example.com
kubeconfig-my-dvp-cluster.example.com
openvpn-admin-my-dvp-cluster.example.com
prometheus-my-dvp-cluster.example.com
status-my-dvp-cluster.example.com
upmeter-my-dvp-cluster.example.com
```

Для тестовых целей возможно добавить записи в файл `/etc/hosts` на локальной машине (для Windows  `%SystemRoot%\system32\drivers\etc\hosts`).

Например, для Linux можно воспользоваться командами:

{% snippetcut %}
```shell
export PUBLIC_IP="<PUBLIC_IP>"
export CLUSTER_DOMAIN="my-dvp-cluster.example.com"
sudo -E bash -c "cat <<EOF >> /etc/hosts
$PUBLIC_IP api.$CLUSTER_DOMAIN
$PUBLIC_IP argocd.$CLUSTER_DOMAIN
$PUBLIC_IP dashboard.$CLUSTER_DOMAIN
$PUBLIC_IP documentation.$CLUSTER_DOMAIN
$PUBLIC_IP dex.$CLUSTER_DOMAIN
$PUBLIC_IP grafana.$CLUSTER_DOMAIN
$PUBLIC_IP hubble.$CLUSTER_DOMAIN
$PUBLIC_IP istio.$CLUSTER_DOMAIN
$PUBLIC_IP istio-api-proxy.$CLUSTER_DOMAIN
$PUBLIC_IP kubeconfig.$CLUSTER_DOMAIN
$PUBLIC_IP openvpn-admin.$CLUSTER_DOMAIN
$PUBLIC_IP prometheus.$CLUSTER_DOMAIN
$PUBLIC_IP status.$CLUSTER_DOMAIN
$PUBLIC_IP upmeter.$CLUSTER_DOMAIN
EOF
"
```
{% endsnippetcut %}


## Создание пользователя

Для доступа в веб-интерфейсы кластера можно создать статического пользователя:

1. Сгенерируйте пароль

  ```shell
  echo "<USER-PASSWORD>" | htpasswd -BinC 10 "" | cut -d: -f2 | base64 -w0
  ```

  здесь `<USER-PASSWORD>` — пароль, который нужно установить пользователю.

2. Создайте пользователя


```shell
sudo -i d8 k create -f - <<EOF
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  subjects:
  - kind: User
    name: admin@deckhouse.io
  accessLevel: SuperAdmin
  portForwarding: true
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@my-dvp-cluster.example.com
  password: '<BASE64 СТРОКА С ПРЕДЫДУЩЕГО ШАГА>'

EOF
```

Теперь возможен вход по почте и паролю в веб интерфейсы кластера. Далее рекомендуется настроить пользователей по разделу [Разграничение доступа / Ролевая модель](../../platform-management/access-control/role-model.html).
