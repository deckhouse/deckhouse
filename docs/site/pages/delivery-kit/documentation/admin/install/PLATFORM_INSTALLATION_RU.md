---
title: "Установка платформы"
permalink: ru/delivery-kit/documentation/admin/install.html
lang: ru
---

## Установка Deckhouse Delivery Kit

Начиная с версии 0.10 Deckhouse CLI, установить её можно с помощью [trdl](https://ru.trdl.dev/). Если установка выполняется внутри кластера, включите Deckhouse Tools и следуйте инструкциям интерфейса.

{% alert %}
Если у вас установлена версия ниже 0.10, то её необходимо предварительно удалить.

Если вам нужно установить одну из версий ниже 0.10, воспользуйтесь [устаревшим способом установки](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.67/deckhouse-cli/#how-do-i-install-deckhouse-cli).
{% endalert %}

1. Установите [клиент trdl](https://ru.trdl.dev/quickstart.html#%D1%83%D1%81%D1%82%D0%B0%D0%BD%D0%BE%D0%B2%D0%BA%D0%B0-%D0%BA%D0%BB%D0%B8%D0%B5%D0%BD%D1%82%D0%B0).

1. Добавьте репозиторий Deckhouse CLI в trdl:

   ```bash
   URL=https://trrr.flant.dev/trdl-deckhouse-cli
   ROOT_VERSION=0
   ROOT_SHA512=$(curl -Ls ${URL}/root.json | sha512sum | tr -d '\-[:space:]\n')
   REPO=trdl-d8
   
   trdl add $REPO $URL $ROOT_VERSION $ROOT_SHA512
   ```

1. Установите актуальный стабильный релиз:

   ```bash
   trdl update $REPO $ROOT_VERSION stable
   ```

1. Убедитесь, что исполняемый файл `d8` установлен и работоспособен:

   ```bash
   . $(trdl use $REPO $ROOT_VERSION stable) && d8 --version
   ```

{% alert level="warning" %}
Если вы используете macOS, вам может потребоваться удалить атрибут карантина с исполняемого файла, чтобы Gatekeeper не блокировал его.
(`sudo xattr -d com.apple.quarantine /path/to/d8`)
{% endalert %}
