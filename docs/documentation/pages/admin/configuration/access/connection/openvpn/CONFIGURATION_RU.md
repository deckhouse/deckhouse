---
title: "Настройка"
permalink: ru/admin/configuration/access/connection/openvpn/configuration.html
lang: ru
---

## Доступ к OpenVPN снаружи кластера

Настроить публикацию OpenVPN-сервера можно несколькими способами:

1. Выберите для подключения один или несколько внешних IP-адресов.
1. Воспользуйтесь одним из методов подключения:
- по внешнему IP-адресу (`ExternalIP`) — если есть узлы с публичными IP-адресами;
- с помощью `LoadBalancer` — для всех облачных провайдеров и их схем размещения с поддержкой заказа LoadBalancer;
- `Direct` — настройте путь трафика вручную: от точки входа в кластер до пода с OpenVPN.

### Пример для кластеров bare metal

Примените в кластере следующий YAML-файл:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: openvpn
spec:
  version: 2
  enabled: true
  settings:
    inlet: ExternalIP
    externalIP: 5.4.54.4 # Внешний IP-адрес
```

### Пример для AWS и Google Cloud

Примените в кластере следующий YAML-файл:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: openvpn
spec:
  version: 2
  enabled: true
  settings:
    inlet: LoadBalancer
```

### Пример для публичного IP-адреса на внешнем балансировщике

Примените в кластере следующий YAML-файл:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: openvpn
spec:
  version: 2
  enabled: true
  settings:
    externalHost: 5.4.54.4
    externalIP: 192.168.0.30 # Внутренний IP-адрес, который примет трафик от внешнего балансировщика.
    inlet: ExternalIP
    nodeSelector:
      kubernetes.io/hostname: node
```

## Доступные ресурсы кластера после подключения к VPN

На компьютер пользователя после подключения к VPN доставляются (push) следующие параметры:

- адрес `kube-dns` добавляется в DNS-серверы клиента для возможности прямого обращения к сервисам Kubernetes по FQDN;
- маршрут в локальную сеть;
- маршрут в сервисную сеть кластера;
- маршрут в сеть подов.

## Аудит пользовательских соединений

Для мониторинга пользовательских соединений возможно включить логирование пользовательской активности через VPN в JSON-формате. Группировка трафика происходит по полям `src_ip`, `dst_ip`, `src_port`, `dst_port`, `ip_proto`.
С помощью [log-shipper](../TODO) логи контейнера можно собрать и отправить на хранение для последующего аудита.

## Аутентификация

Для доступа в OpenVPN возможно настроить [аутентификацию](../../authentication/) пользователей. Также можно настроить аутентификацию с помощью параметра [externalAuthentication](#TODO). Если эти варианты отключены, будет включена базовая аутентификация со сгенерированным паролем.

Чтобы просмотреть сгенерированный пароль, выполните команду:

```shell
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values openvpn -o json | jq '.openvpn.internal.auth.password'
```

Чтобы сгенерировать новый пароль, удалите ресурс Secret:

```shell
kubectl -n d8-openvpn delete secret/basic-auth
```

{% alert level="info" %}
Параметр `auth.password` больше не поддерживается.
{% endalert %}

## Почему не работает автоматическая настройка DNS-сервера при подключении на macOS и Linux с помощью клиента OpenVPN?

В связи с архитектурными особенностями операционных систем семейства Linux и macOS автоматическая конфигурация DNS-сервера при подключении с помощью официального клиента OpenVPN невозможна.

Для настройки DNS-сервера в таких ОС сервисом предусмотрена возможность использования сторонних скриптов, которые запускаются при подключении и отключении клиента.

В клиентских конфигурациях, генерируемых модулем, предопределены и закомментированы блоки, отвечающие за эти настройки:

```bash
# Uncomment the lines below for use with Linux
#script-security 2
# If you use resolved
#up /etc/openvpn/update-resolv-conf
#down /etc/openvpn/update-resolv-conf
# If you use systemd-resolved, first install the openvpn-systemd-resolved package
#up /etc/openvpn/update-systemd-resolved
#down /etc/openvpn/update-systemd-resolved
```

Для активации указанных блоков кода необходимо их раскомментировать (удалить начальный символ `#`), а также указать корректные пути к скриптам.

Скрипты можно подготовить самостоятельно или воспользоваться готовыми решениями от официального [OpenVPN Community](https://community.openvpn.net/openvpn/wiki/Pushing-DNS-to-clients) (для Linux).
Для macOS можно воспользоваться [сторонним скриптом](https://github.com/andrewgdotcom/openvpn-mac-dns/blob/master/etc/openvpn/update-resolv-conf).

{% alert level="warning" %}
Скрипты должны обладать правами на исполнение.
{% endalert %}
