---
title: "Модуль openvpn: FAQ"
---

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

## Как отозвать, ротировать или удалить сертификат клиента?

Все действия с сертификатами клиентов производится из веб-интерфейса `openvpn-admin` путем нажатия на кнопки находящиеся справа от каждого пользователя.
![Действия с активным пользователем](../../images/openvpn/active_user.png)

Ротировать сертификат или удалить клиента можно только после его отзыва (Revoke).  
![Действия с отозванным пользователем](../../images/openvpn/revoked_user.png)

## Как ротировать сертификат сервера?

Сертификат сервера ротируется автоматически за N дней до конца срока действия.  

Для принудительной ротации требуется выполнить следующие действия:
* Удалить секрет `openvpn-pki-server` в namespace `d8-openvpn`

```shell
kubectl -n d8-openvpn delete secrets openvpn-pki-server
```

* Перезапустить поды openvpn

```shell
kubectl -n d8-openvpn rollout restart sts openvpn
```

## Как ротировать корневой сертификат (CA)?

С помощью корневого сертификата OpenVPN изданы сертификаты для сервера и всех клиентов.  
По этой причине, при замене корневого сертификата требуется перевыпустить все указанные сертификаты.  

Для ротации требуется выполнить следующие действия:
* [Отозвать или удалить](#как-отозвать-ротировать-или-удалить-сертификат-клиента) все активные клиентские сертификаты.  
Если использовать отзыв клиента, то после замены корневого сертификат можно выполнить ротацию (renew) сертификата пользователя. Это избавит от необходимости повторного создания клиента
* Удалить секреты `openvpn-pki-ca` и `openvpn-pki-server`  в namespace `d8-openvpn`

```shell
kubectl -n d8-openvpn delete secrets openvpn-pki-ca openvpn-pki-server
```

* Перезапустить поды OpenVPN

```shell
kubectl -n d8-openvpn rollout restart sts openvpn
```

* [Ротировать сертификаты](#как-отозвать-ротировать-или-удалить-сертификат-клиента) отозванных пользователей, либо создать новых.
* Удалить все отозванные сертификаты

```shell
kubectl  -n d8-openvpn delete secrets -l revokedForever=true
```
