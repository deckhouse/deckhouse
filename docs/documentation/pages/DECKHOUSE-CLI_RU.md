---
title: Deckhouse CLI
permalink: ru/deckhouse-cli/
lang: ru
---

Deckhouse CLI — это интерфейс командной строки для работы с кластерами Deckhouse Kubernetes Platform (DKP). По умолчанию он устанавливается с платформой.

Утилита называется `d8`. 

Три компонента интерфейса отвечают:
* `d8 k` — за команды, которые в кластерах Kubernetes выполняет `kubectl`.
    Например, в кластере DKP можно выполнить `kubectl get pods` как `d8 k get pods`.
* `d8 d` — 
* `d8 v` — 

Качай Deckhouse CLI:
- [Linux x86-64]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/downloads/deckhouse-cli/v0.0.3/d8-v0.0.3-linux-amd64.tar.gz)
- [macOS x86-64]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/downloads/deckhouse-cli/v0.0.3/d8-v0.0.3-darwin-amd64.tar.gz)
- [macOS ARM64]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/downloads/deckhouse-cli/v0.0.3/d8-v0.0.3-darwin-arm64.tar.gz)
