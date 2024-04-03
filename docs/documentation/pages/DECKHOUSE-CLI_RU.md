---
title: Deckhouse CLI
permalink: ru/deckhouse-cli/
lang: ru
---

Deckhouse CLI — это интерфейс командной строки для работы с кластерами Deckhouse Kubernetes Platform (DKP), который устанавливается вместе с платформой. Утилиту можно установить и [вручную](#manual).

Утилита называется `d8`. 

Три компонента интерфейса отвечают:
* `d8 k` — за команды, которые в кластерах Kubernetes выполняет `kubectl`.
    Например, в кластере DKP можно выполнить `kubectl get pods` как `d8 k get pods`.
* `d8 d` — за процессы доставки по аналогии с утилитой `werf`.
    Например, вместо `werf plan --repo registry.deckhouse.io` можно выполнить `d8 d plan --repo registry.deckhouse.io`.
* `d8 v` — за работу с виртуальными машинами.
    Например, команда `d8 virtualziation console` подключает к консоли виртуальной машины.

Качай Deckhouse CLI:
- [Linux x86-64]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/downloads/deckhouse-cli/v0.0.3/d8-v0.0.3-linux-amd64.tar.gz)
- [macOS x86-64]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/downloads/deckhouse-cli/v0.0.3/d8-v0.0.3-darwin-amd64.tar.gz)
- [macOS ARM64]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/downloads/deckhouse-cli/v0.0.3/d8-v0.0.3-darwin-arm64.tar.gz)
