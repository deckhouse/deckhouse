---
title: "Прочие настройки"
permalink: ru/virtualization-platform/documentation/admin/install/steps/ingress.html
lang: ru
---

{% alert level="info" %}
Для выполнения приведенных ниже команд необходима установленная утилита [d8](/products/kubernetes-platform/documentation/v1/cli/d8/) (Deckhouse CLI) и настроенный контекст kubectl для доступа к кластеру. Также, можно подключиться к master-узлу по SSH и выполнить команду от пользователя `root` с помощью `sudo -i`.
{% endalert %}

## Настройка Ingress

Убедитесь, что под Kruise controller manager модуля [ingress-nginx](/modules/ingress-nginx/) запустился и находится в статусе `Running`:

```shell
d8 k -n d8-ingress-nginx get po -l app=kruise
```

Создайте ресурс IngressNginxController, описывающий параметры Ingress NGINX Controller:

```yaml
d8 k apply -f - <<EOF
# Секция, описывающая параметры Ingress NGINX Controller.
# https://deckhouse.ru/modules/ingress-nginx/cr.html#ingressnginxcontroller
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

Запуск Ingress-контроллера может занять какое-то время. Убедитесь, что поды Ingress-контроллера перешли в статус `Running`, выполнив команду:

```shell
d8 k -n d8-ingress-nginx get po -l app=controller
```

{% offtopic title="Пример вывода..." %}

```console
NAME                                       READY   STATUS    RESTARTS   AGE
controller-nginx-r6hxc                     3/3     Running   0          5m
```

{% endofftopic %}

## Настройка DNS

Для доступа к веб-интерфейсам платформы необходимо настроить DNS-записи для домена кластера.

{% alert level="warning" %}
Домен, используемый в шаблоне, не должен совпадать с доменом, указанным в параметре `clusterDomain`, и внутренней сервисной зоной сети. Например, если используется `clusterDomain: cluster.local` (значение по умолчанию),, а сервисная зона сети — `ru-central1.internal`, то `publicDomainTemplate` не может быть `%s.cluster.local` или `%s.ru-central1.internal`.
{% endalert %}

### Использование Wildcard-домена

Убедитесь, что поддомены разрешаются на IP-адрес узла, на котором работает Ingress controller. Также проверьте, что шаблон имён соответствует формату `%s.<домен>`:

```shell
d8 k get mc global -ojson | jq -r '.spec.settings.modules.publicDomainTemplate'
```

Пример вывода, если использовался свой wildcard-домен:

```console
%s.my-dvp-cluster.example.com
```

Пример вывода, если использовался домен от сервиса sslip.io:

```console
%s.54.43.32.21.sslip.io
```

### Использование отдельных доменов вместо wildcard-домена

Если в шаблоне используется не wildcard-домен, необходимо вручную добавить дополнительные A или CNAME-записи, указывающие на публичный IP-адрес узла, где работает nginx-controller. Эти записи требуются для всех сервисов Deckhouse.

Например, для домена `my-dvp-cluster.example.com` и шаблона с поддоменами `%s.my-dvp-cluster.example.com`, записи будут выглядеть так:

```console
api.my-dvp-cluster.example.com
console.my-dvp-cluster.example.com
dashboard.my-dvp-cluster.example.com
deckhouse-tools.my-dvp-cluster.example.com
dex.my-dvp-cluster.example.com
documentation.my-dvp-cluster.example.com
grafana.my-dvp-cluster.example.com
hubble.my-dvp-cluster.example.com
kubeconfig.my-dvp-cluster.example.com
openvpn-admin.my-dvp-cluster.example.com
prometheus.my-dvp-cluster.example.com
status.my-dvp-cluster.example.com
upmeter.my-dvp-cluster.example.com
```

Для домена `my-dvp-cluster.example.com` и шаблона с индивидуальными доменами `%s-my-dvp-cluster.example.com`, записи будут выглядеть так:

```console
api-my-dvp-cluster.example.com
console-my-dvp-cluster.example.com
dashboard-my-dvp-cluster.example.com
deckhouse-tools-my-dvp-cluster.example.com
dex-my-dvp-cluster.example.com
documentation-my-dvp-cluster.example.com
grafana-my-dvp-cluster.example.com
hubble-my-dvp-cluster.example.com
kubeconfig-my-dvp-cluster.example.com
openvpn-admin-my-dvp-cluster.example.com
prometheus-my-dvp-cluster.example.com
status-my-dvp-cluster.example.com
upmeter-my-dvp-cluster.example.com
```

Для тестирования можно добавить необходимые записи в файл `/etc/hosts` на локальной машине (для Windows в файл `%SystemRoot%\system32\drivers\etc\hosts`).

Для Linux можно использовать следующие команды для добавления записей в файл `/etc/hosts`:

```shell
export PUBLIC_IP="<PUBLIC_IP>"
export CLUSTER_DOMAIN="my-dvp-cluster.example.com"
sudo -E bash -c "cat <<EOF >> /etc/hosts
$PUBLIC_IP api.$CLUSTER_DOMAIN
$PUBLIC_IP console.$CLUSTER_DOMAIN
$PUBLIC_IP dashboard.$CLUSTER_DOMAIN
$PUBLIC_IP deckhouse-tools.$CLUSTER_DOMAIN
$PUBLIC_IP dex.$CLUSTER_DOMAIN
$PUBLIC_IP documentation.$CLUSTER_DOMAIN
$PUBLIC_IP grafana.$CLUSTER_DOMAIN
$PUBLIC_IP hubble.$CLUSTER_DOMAIN
$PUBLIC_IP kubeconfig.$CLUSTER_DOMAIN
$PUBLIC_IP openvpn-admin.$CLUSTER_DOMAIN
$PUBLIC_IP prometheus.$CLUSTER_DOMAIN
$PUBLIC_IP status.$CLUSTER_DOMAIN
$PUBLIC_IP upmeter.$CLUSTER_DOMAIN
EOF
"
```

## Создание пользователя

Для доступа в веб-интерфейсы кластера можно создать статического пользователя:

1. Сгенерируйте пароль:

   ```shell
   echo -n '<USER-PASSWORD>' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
   ```

   `<USER-PASSWORD>` — пароль, который нужно установить пользователю.

1. Создайте пользователя:

   ```shell
   d8 k create -f - <<EOF
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

Теперь можно авторизоваться в веб-интерфейсах кластера, используя электронную почту и пароль. Для дальнейшей настройки рекомендуется ознакомиться с разделом [Разграничение доступа / Ролевая модель](../../platform-management/access-control/role-model.html).

## Включение модуля console

{% alert level="info" %}
Модуль доступен только только пользователям EE-редакции.
{% endalert %}

Модуль `console` позволит управлять компонентами виртуализации через веб-интерфейс Deckhouse.

Чтобы включить модуль, выполните следующую команду:

```shell
d8 system module enable console
```
