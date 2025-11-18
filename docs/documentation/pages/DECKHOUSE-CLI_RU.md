---
title: "Описание и установка Deckhouse CLI"
permalink: ru/cli/d8/
description: Deckhouse CLI — интерфейс командной строки для работы с кластерами от разработчиков Deckhouse.
lang: ru
---

Deckhouse CLI — это интерфейс командной строки для работы с кластерами от Deckhouse Platform Certified Security Edition. Начиная с релиза 1.59, интерфейс автоматически устанавливается на все узлы кластера. Утилиту можно также [установить](#как-установить-deckhouse-cli) на любую машину и использовать для работы с кластерами без Deckhouse Platform Certified Security Edition.

В командной строке к утилите можно обратиться как `d8`. Все команды сгруппированы по функциям:

* `d8 k` — команды, которые в кластерах Kubernetes выполняет `kubectl`.  
    Например, в кластере можно выполнить `kubectl get pods` как `d8 k get pods`.
* `d8 dk` — команды, отвечающие за доставку по аналогии с утилитой `werf`.  
    Например, вместо `werf plan --repo registry.deckhouse.ru` можно выполнить `d8 d plan --repo registry.deckhouse.ru`.

* `d8 mirror` — команды, которые позволяют скопировать образы дистрибутива Deckhouse Platform Certified Security Edition в частный container registry (ранее для этого использовалась утилита `dhctl mirror`).
  Например, можно выполнить `d8 mirror pull -l <LICENSE> <TAR-BUNDLE-PATH>` вместо `dhctl mirror --license <LICENSE> --images-bundle-path <TAR-BUNDLE-PATH>`.

  Флаг `--only-extra-images` позволяет загружать только дополнительные образы для модулей (например, базы данных уязвимостей) без загрузки основных образов модулей.

  Сценарии использования:

  - ручная загрузка образов в изолированный приватный registry.
  - Обновление дополнительных образов модулей (например, баз данных уязвимостей): `d8 mirror pull --include-module <module-name> --only-extra-images bundle.tar`

* `d8 v` — команды, отвечающие за работу с виртуальными машинами.  
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

* `d8 backup` — команды для создания резервных копий ключевых компонентов кластера:

  * `etcd` — полная резервная копия ключевого хранилища etcd;
  * `cluster-config` — архив конфигурационных объектов;
  * `loki` — диагностическая выгрузка логов из встроенного Loki API (не предназначена для восстановления).

    Например:

    ```console
    d8 backup etcd ./etcd.snapshot
    d8 backup cluster-config ./cluster-config.tar
    d8 backup loki --days 1 > ./loki.log
    ```

    Список доступных флагов `d8 backup` можно получить через команду `d8 backup --help`.

## Как установить Deckhouse CLI

{% include d8-cli-install/main.liquid %}
