---
title: Deckhouse CLI
permalink: ru/deckhouse-cli/
lang: ru
---

Deckhouse CLI — это интерфейс командной строки для работы с кластерами от Deckhouse Kubernetes Platform (DKP). Начиная с релиза 1.59, интерфейс автоматически устанавливается на все узлы кластера. Утилиту можно также [установить](#как-установить-deckhouse-cli) на любую машину и использовать для работы с кластерами без DKP.

В командной строке к утилите можно обратиться как `d8`. Все команды сгруппированы по функциям:
* `d8 k` — команды, которые в кластерах Kubernetes выполняет `kubectl`.  
    Например, в кластере можно выполнить `kubectl get pods` как `d8 k get pods`.
* `d8 d` — команды, отвечающие за доставку по аналогии с утилитой `werf`.  
    Например, вместо `werf plan --repo registry.deckhouse.io` можно выполнить `d8 d plan --repo registry.deckhouse.io`.

    {% alert level="warning" %}
    Группа команд `d8 d` не доступна для редакции Community Edition (CE).
    {% endalert %}

* `d8 v` — команды, отвечающие за работу с виртуальными машинами, созданными [Deckhouse Virtualization Platform](/modules/virtualization/stable/).  
    Например, команда `d8 virtualization console` подключает к консоли виртуальной машины.

    <div markdown="0">
    <details><summary>Больше команд для виртуализации...</summary>
    <ul>
    <li><code>d8 v console</code> подключает к консоли виртуальной машины.</li>
    <li><code>d8 v port-forward</code> перенаправляет локальные порты на виртуальную машину.</li>
    <li><code>d8 v scp</code> использует клиент SCP для работы с файлами на виртуальной машине.</li>
    <li><code>d8 v ssh</code> подключает к виртуальной машине по SSH.</li>
    <li><code>d8 v vnc</code> подключает к виртуальной машине по VNC.</li>
    </ul>
    </details>
    </div>

## Как установить Deckhouse CLI?

1. Скачайте архив для вашей ОС:
   * [Linux x86-64]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/downloads/deckhouse-cli/v{{ site.data.deckhouse-cli.version }}/d8-v{{ site.data.deckhouse-cli.version }}-linux-amd64.tar.gz)
   * [macOS x86-64]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/downloads/deckhouse-cli/v{{ site.data.deckhouse-cli.version }}/d8-v{{ site.data.deckhouse-cli.version }}-darwin-amd64.tar.gz)
   * [macOS ARM64]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/downloads/deckhouse-cli/v{{ site.data.deckhouse-cli.version }}/d8-v{{ site.data.deckhouse-cli.version }}-darwin-arm64.tar.gz)

1. Распакуйте архив:

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
