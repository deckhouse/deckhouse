---
title: Выгрузка образов модулей DKP из репозитория вендора
permalink: ru/update/security/vendor-images/
lang: ru
---

Для выгрузки образов модулей DKP из репозитория вендора,  сделайте следующие шаги:

1. Создайте зашифрованную base64 строку для доступа клиента Docker в репозиторий вендора. Сделать это можно, например, командой ниже, заменив `YOUR_USERNAME` на `license-token`, а `YOUR_PASSWORD` — на ваш лицензионный ключ:

   ```bash
   base64 -w0 <<EOF
     {
       "auths": {
         "registry.deckhouse.ru": {
           "auth": "$(echo -n 'YOUR_USERNAME:YOUR_PASSWORD' | base64 -w0)"
         }
       }
     }
   EOF
   ```

2. Создайте в текущем каталоге файл `ModuleSource`, например, `ms.yml` следующего содержания:

   `ms.yml`

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleSource
   metadata:
     name: deckhouse
   spec:
     registry:
    # Укажите строку, полученную в п.1 вместо CHANGE
       dockerCfg: CHANGE
       repo: registry.deckhouse.ru/deckhouse/ee/modules
       scheme: HTTPS
     # Выберите подходящий канал обновлений: Alpha, Beta, EarlyAccess, Stable, RockSolid
     releaseChannel: "Stable"
   ```

3. Запустите загрузку модулей DKP из репозитория вендора в локальный каталог рабочей станции:

   ```bash
   dhctl mirror-modules --modules-dir=$(pwd)/d8-modules --module-source=$(pwd)/ms.yml
   ```

В результате работы утилиты в каталог `d8-modules` будут сохранены все необходимые артефакты, необходимые для переноса модулей DKP в закрытое окружение. Примерный объём данных составляет 7 Гб.

4. Выполните перенос на рабочую станцию в закрытом окружении следующих элементов:

- каталога `d8-modules`;
- исполняемого файла `dhctl`.
