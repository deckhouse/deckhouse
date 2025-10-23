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

### Пример для bare-metal кластеров

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
    externalIP: 5.4.54.4 # Внешний IP-адрес.
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

## Аутентификация

Для доступа в OpenVPN возможно настроить [аутентификацию](../../authentication/) пользователей. Также можно настроить аутентификацию с помощью параметра [externalAuthentication](/modules/openvpn/configuration.html#parameters-auth-externalauthentication). Если эти варианты отключены, будет включена базовая аутентификация со сгенерированным паролем.

Чтобы просмотреть сгенерированный пароль, выполните команду:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values openvpn -o json | jq '.openvpn.internal.auth.password'
```

Чтобы сгенерировать новый пароль, удалите секрет:

```shell
d8 k -n d8-openvpn delete secret/basic-auth
```

{% alert level="info" %}
Параметр `auth.password` больше не поддерживается.
{% endalert %}

## Доступные ресурсы кластера после подключения к VPN

На компьютер пользователя после подключения к VPN доставляются (push) следующие параметры:

- Адрес `kube-dns` добавляется в DNS-серверы клиента для возможности прямого обращения к сервисам Kubernetes по FQDN;
- Маршрут в локальную сеть;
- Маршрут в сервисную сеть кластера;
- Маршрут в сеть подов.

## Аудит пользовательских соединений

Для мониторинга пользовательских соединений возможно включить логирование пользовательской активности через VPN в JSON-формате. Группировка трафика происходит по полям `src_ip`, `dst_ip`, `src_port`, `dst_port`, `ip_proto`.
С помощью [log-shipper](/modules/log-shipper/) логи контейнера можно собрать и отправить на хранение для последующего аудита.

## Почему не работает автоматическая настройка DNS-сервера при подключении на macOS и Linux с помощью клиента OpenVPN

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

Скрипты можно подготовить самостоятельно, или воспользоваться готовыми решениями от официального [OpenVPN Community](https://community.openvpn.net/openvpn/wiki/Pushing-DNS-to-clients) (для Linux).
Для macOS можно воспользоваться [сторонним скриптом](https://github.com/andrewgdotcom/openvpn-mac-dns/blob/master/etc/openvpn/update-resolv-conf).

{% alert level="warning" %}
Скрипты должны обладать правами на исполнение.
{% endalert %}

## Как отозвать, ротировать или удалить сертификат клиента

Все действия с клиентскими сертификатами выполняются через веб-интерфейс `openvpn-admin`. Справа от имени каждого пользователя доступны кнопки для управления сертификатом:

![Действия с активным пользователем](../../../../../images/openvpn/active_user.png)

Чтобы ротировать (выпустить новый сертификат) или удалить клиента, необходимо сначала отозвать его текущий сертификат (Revoke):

![Действия с отозванным пользователем](../../../../../images/openvpn/revoked_user.png)

После отзыва становятся доступными действия Renew (ротация) и Delete (удаление).

## Как ротировать сертификат сервера

Серверный сертификат ротируется автоматически за 1 день до окончания срока его действия.  

Если требуется выполнить ротацию вручную (например, при повреждении сертификата или внеплановой замене), выполните следующие шаги:

1. Удалите текущий секрет, содержащий сертификат и ключ сервера:

   ```shell
   d8 k -n d8-openvpn delete secrets openvpn-pki-server
   ```

1. Перезапустите под OpenVPN, чтобы инициировать генерацию нового сертификата:

   ```shell
   d8 k -n d8-openvpn rollout restart sts openvpn
   ```

## Как ротировать корневой сертификат (CA)

Корневой сертификат (CA) и серверный сертификат ротируется автоматически за 1 день до окончания срока действия. Автоматическая ротация сертификатов пользователя не предусмотрена.
Корневой сертификат (CA) используется для подписи всех сертификатов в OpenVPN — как серверных, так и клиентских. Поэтому при его замене необходимо перевыпустить все зависимые сертификаты.

Шаги для ротации корневого сертификата:

1. [Отзовите или удалите](#как-отозвать-ротировать-или-удалить-сертификат-клиента) все активные клиентские сертификаты. Сделать это можно через интерфейс `openvpn-admin`. Если вы воспользуетесь отзывом, то после замены CA можно будет выполнить ротацию сертификатов (Renew), не создавая клиента заново.

1. Удалите секреты `openvpn-pki-ca` и `openvpn-pki-server`  в пространстве имён `d8-openvpn`:

   ```shell
   d8 k -n d8-openvpn delete secrets openvpn-pki-ca openvpn-pki-server
   ```

1. Перезапустите поды OpenVPN:

   ```shell
   d8 k -n d8-openvpn rollout restart sts openvpn
   ```

1. Выполните [ротацию сертификатов](#как-отозвать-ротировать-или-удалить-сертификат-клиента) для отозванных клиентов или создайте новых клиентов с новыми сертификатами.

1. Удалите все отозванные сертификаты:

   ```shell
   d8 k  -n d8-openvpn delete secrets -l revokedForever=true
   ```
