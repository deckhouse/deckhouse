---
title: Bastion-хост
description: Хост, через который выполняется доступ по SSH к master-узлу, когда у узлов кластера нет публичных IP-адресов.
lang: ru
---

Bastion-хост нужен в [схемах размещения](glossary.html#layout), в которых узлы создаются без публичных IP-адресов. [Установщик](glossary.html#installer) подключается через него, если запущен с параметрами `--ssh-bastion-host`, `--ssh-bastion-user` и, при необходимости, `--ssh-bastion-port`.
