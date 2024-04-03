---
title: Deckhouse CLI
permalink: ru/deckhouse-cli/
lang: ru
---

Deckhouse CLI — это интерфейс командной строки для работы с кластерами Deckhouse Kubernetes Platform (DKP), который автоматически устанавливается вместе с платформой. Утилиту можно дополнительно [установить](#как-установить-deckhouse-cli) на любую машину.

Утилита называется `d8` и состоит из трех компонентов:
* `d8 k` — команды, которые в кластерах Kubernetes выполняет `kubectl`.  
    Например, в кластере DKP можно выполнить `kubectl get pods` как `d8 k get pods`.
* `d8 d` — команды, отвечающие за доставку, похожие на работу утилиты `werf`.  
    Например, вместо `werf plan --repo registry.deckhouse.io` можно выполнить `d8 d plan --repo registry.deckhouse.io`.
* `d8 v` — команды, отвечающие за работу с виртуальными машинами.  
    Например, команда `d8 virtualziation console` подключает к консоли виртуальной машины.

    <div markdown="0">
    <details><summary>Больше команд</summary>
    <ul>
    <li><code>d8 v console</code> подключает к консоли виртуальной машины.</li>
    <li><code>d8 v port-forward</code> перенаправляет локальные порты на виртуальную машину.</li>
    <li><code>d8 v scp</code> использует клиент SCP для работы с файлами на виртуальной машине.</li>
    <li><code>d8 v ssh</code> настроит SSH-соединение с виртуальной машиной.</li>
    <li><code>d8 v vnc</code> настроит VNC-соединение с виртуальной машиной.</li>
    </ul>
    </details>
    </div>

## Как установить Deckhouse CLI?

1. Скачайте архив с подходящей версией ОС:
    * [Linux x86-64]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/downloads/deckhouse-cli/v0.0.3/d8-v0.0.3-linux-amd64.tar.gz)
    * [macOS x86-64]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/downloads/deckhouse-cli/v0.0.3/d8-v0.0.3-darwin-amd64.tar.gz)
    * [macOS ARM64]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/downloads/deckhouse-cli/v0.0.3/d8-v0.0.3-darwin-arm64)

1. Распакуйте архив командой:

   ```bash
   tar -xvf "d8-v${RELEASE_VERSION}-${OS}-${ARCH}.tar.gz" "${OS}-${ARCH}/d8"
   ```

1. Переместите файл `d8` в каталог в переменной `PATH` вашей системы:

   ```bash
   sudo mv "${OS}-${ARCH}/d8" /usr/local/bin/
   ```

1. Проверьте, что утилита работает:

   ```bash
   d8 help
   ```

Готово, вы установили Deckhouse CLI.
