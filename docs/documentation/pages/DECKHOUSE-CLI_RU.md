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
    Например, вместо `werf plan --repo registry.deckhouse.io` можно выполнить `d8 d plan --repo registry.deckhouse.io`.

* `d8 mirror` — команды, которые позволяют скопировать образы дистрибутива Deckhouse Platform Certified Security Edition в частный container registry (ранее для этого использовалась утилита `dhctl mirror`).
  Например, можно выполнить `d8 mirror pull -l <LICENSE> <TAR-BUNDLE-PATH>` вместо `dhctl mirror --license <LICENSE> --images-bundle-path <TAR-BUNDLE-PATH>`.

  Флаг `--only-extra-images` позволяет загружать только дополнительные образы для модулей (например, базы данных уязвимостей) без загрузки основных образов модулей.

  Сценарии использования:

  - [ручная загрузка образов в изолированный приватный registry](../../installing/#ручная-загрузка-образов-deckhouse-kubernetes-platform-бд-сканера-уязвимостей-и-модулей-dkp-в-приватный-registry).
  - Обновление дополнительных образов модулей (например, баз данных уязвимостей): `d8 mirror pull --include-module <module-name> --only-extra-images bundle.tar`

* `d8 v` — команды, отвечающие за работу с виртуальными машинами, созданными [Deckhouse Virtualization Platform](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html).  
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

Установить Deckhouse CLI возможно двумя способами:

* Начиная с версии 0.10 доступна установка с помощью [trdl](https://ru.trdl.dev/). Такой способ позволяет непрерывно получать свежие версии утилиты со всеми доработками и исправлениями.
  > Обратите внимание, что для установки через trdl необходим доступ в Интернет к tuf-репозиторию с утилитой. В кластере с закрытым окружением такой способ работать не будет!
* Вручную скачав исполняемый файл и установив его в системе.

### Установка с помощью trdl

Начиная с версии 0.10 Deckhouse CLI установить её можно с помощью [trdl](https://ru.trdl.dev/).

{% alert level="warning" %}
Если у вас установлена версия ниже 0.10, то её необходимо предварительно удалить.

Если вам нужно установить одну из версий ниже 0.10, воспользуйтесь [устаревшим способом установки](#установка-исполняемого-файла).
{% endalert %}

1. Установите [клиент trdl](https://ru.trdl.dev/quickstart.html#%D1%83%D1%81%D1%82%D0%B0%D0%BD%D0%BE%D0%B2%D0%BA%D0%B0-%D0%BA%D0%BB%D0%B8%D0%B5%D0%BD%D1%82%D0%B0).

1. Добавьте репозиторий Deckhouse CLI в trdl:

   ```bash
   URL=https://deckhouse.ru/downloads/deckhouse-cli-trdl
   ROOT_VERSION=1
   ROOT_SHA512=343bd5f0d8811254e5f0b6fe292372a7b7eda08d276ff255229200f84e58a8151ab2729df3515cb11372dc3899c70df172a4e54c8a596a73d67ae790466a0491
   REPO=d8

   trdl add $REPO $URL $ROOT_VERSION $ROOT_SHA512
   ```

1. Установите последний стабильный выпуск утилиты `d8` и проверьте ее работоспособность:

   ```bash
   . $(trdl use d8 0 stable) && d8 --version
   ```

Если вы не хотите вызывать `. $(trdl use d8 0 stable)` перед каждым использованием Deckhouse CLI, добавьте строку `alias d8='trdl exec d8 0 stable -- "$@"'` в RC-файл вашей командной оболочки.

Готово, вы установили Deckhouse CLI.

### Установка исполняемого файла

{% include d8-cli-install/main.liquid %}
