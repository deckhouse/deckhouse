---
title: Deckhouse CLI
permalink: ru/deckhouse-cli/
description: Deckhouse CLI — интерфейс командной строки для работы с кластерами от разработчиков Deckhouse.
lang: ru
---

Deckhouse CLI — это интерфейс командной строки для работы с кластерами от Deckhouse Kubernetes Platform (DKP). Начиная с релиза 1.59, интерфейс автоматически устанавливается на все узлы кластера. Утилиту можно также [установить](#как-установить-deckhouse-cli) на любую машину и использовать для работы с кластерами без DKP.

В командной строке к утилите можно обратиться как `d8`. Все команды сгруппированы по функциям:

{% alert level="info" %}
Группы команд `d8 d` и `d8 mirror` недоступны для Community Edition (CE) и Basic Edition (BE).
{% endalert %}

* `d8 k` — команды, которые в кластерах Kubernetes выполняет `kubectl`.  
    Например, в кластере можно выполнить `kubectl get pods` как `d8 k get pods`.
* `d8 dk` — команды, отвечающие за доставку по аналогии с утилитой `werf`.  
    Например, вместо `werf plan --repo registry.deckhouse.io` можно выполнить `d8 d plan --repo registry.deckhouse.io`.

* `d8 mirror` — команды, которые позволяют скопировать образы дистрибутива DKP в частный container registry (ранее для этого использовалась утилита `dhctl mirror`).
  Например, можно выполнить `d8 mirror pull -l <LICENSE> <TAR-BUNDLE-PATH>` вместо `dhctl mirror --license <LICENSE> --images-bundle-path <TAR-BUNDLE-PATH>`.

  Сценарии использования:

  - [ручная загрузка образов в изолированный приватный registry](/products/kubernetes-platform/documentation/v1/deckhouse-faq.html#%D1%80%D1%83%D1%87%D0%BD%D0%B0%D1%8F-%D0%B7%D0%B0%D0%B3%D1%80%D1%83%D0%B7%D0%BA%D0%B0-%D0%BE%D0%B1%D1%80%D0%B0%D0%B7%D0%BE%D0%B2-%D0%B2-%D0%B8%D0%B7%D0%BE%D0%BB%D0%B8%D1%80%D0%BE%D0%B2%D0%B0%D0%BD%D0%BD%D1%8B%D0%B9-%D0%BF%D1%80%D0%B8%D0%B2%D0%B0%D1%82%D0%BD%D1%8B%D0%B9-registry);
  - [ручная загрузка образов подключаемых модулей Deckhouse в изолированный приватный registry](/products/kubernetes-platform/documentation/v1/deckhouse-faq.html#%D1%80%D1%83%D1%87%D0%BD%D0%B0%D1%8F-%D0%B7%D0%B0%D0%B3%D1%80%D1%83%D0%B7%D0%BA%D0%B0-%D0%BE%D0%B1%D1%80%D0%B0%D0%B7%D0%BE%D0%B2-%D0%BF%D0%BE%D0%B4%D0%BA%D0%BB%D1%8E%D1%87%D0%B0%D0%B5%D0%BC%D1%8B%D1%85-%D0%BC%D0%BE%D0%B4%D1%83%D0%BB%D0%B5%D0%B9-deckhouse-%D0%B2-%D0%B8%D0%B7%D0%BE%D0%BB%D0%B8%D1%80%D0%BE%D0%B2%D0%B0%D0%BD%D0%BD%D1%8B%D0%B9-%D0%BF%D1%80%D0%B8%D0%B2%D0%B0%D1%82%D0%BD%D1%8B%D0%B9-registry).

* `d8 v` — команды, отвечающие за работу с виртуальными машинами, созданными [Deckhouse Virtualization Platform](https://deckhouse.ru/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html).  
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

## Как установить Deckhouse CLI

Установить Deckhouse CLI возможно двумя способами:

* Начиная с версии 0.10 доступна установка с помощью [trdl](https://ru.trdl.dev/). Такой способ позволяет непрерывно получать свежие версии утилиты со всеми доработками и исправлениями.
  > Обратите внимание, что для установки через trdl необходим доступ в Интернет к tuf-репозиторию с утилитой. В кластере с закрытым окружением такой способ работать не будет!
* Вручную скачав исполняемый файл и установив его в системе.

### Установка с помощью trdl

Начиная с версии 0.10 Deckhouse CLI установить её можно с помощью [trdl](https://ru.trdl.dev/).

{% alert %}
Если установка выполняется внутри кластера, включите [Deckhouse Tools](../modules/deckhouse-tools/) и следуйте инструкциям интерфейса.
{% endalert %}

{% alert level="warning" %}
Если у вас установлена версия ниже 0.10, то её необходимо предварительно удалить.

Если вам нужно установить одну из версий ниже 0.10, воспользуйтесь [устаревшим способом установки](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.67/deckhouse-cli/#how-do-i-install-deckhouse-cli).
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

{% alert level="warning" %}
Если вы используете macOS, вам может потребоваться удалить атрибут карантина с исполняемого файла, чтобы Gatekeeper не блокировал его.
(`sudo xattr -d com.apple.quarantine /path/to/d8`)
{% endalert %}

### Установка исполняемого файла

{% include d8-cli-install/main.liquid %}
