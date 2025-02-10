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
