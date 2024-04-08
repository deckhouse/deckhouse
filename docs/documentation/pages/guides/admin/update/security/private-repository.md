---
title: Загрузка и выгрузка образов модулей DKP
permalink: ru/update/security/private-repository/
lang: ru
---

1. Из каталога рабочей станции в закрытом окружении, содержащего утилиту `dhctl` и каталог с образами модулей DKP `d8-modules`, выполните загрузку образов в закрытый репозиторий следующей командой:

   ```bash
   dhctl mirror-modules \
    --modules-dir=$(pwd)/d8-modules \
    --registry="registry.example.com:5000/deckhouse/ee/modules" \
    --registry-login="YOUR_USERNAME" \
    --registry-password="YOUR_PASSWORD"
   ```

2. Если ваш репозиторий не требует авторизации, не указывайте флаги `--registry-login` / `--registry-password`.

3. Укажите верный путь в репозитории: там должна находиться поставка DKP. В примере выше поменяйте `/deckhouse/ee` на правильный путь размещения образов DKP.

4. Проверьте, что `ModuleSource` с названием `deckhouse` в вашем кластере указывает на верный путь до модулей (`spec.registry.repo`), а также в нем нет ошибок (`status.moduleErrors`).

   ```bash
   kubectl get ms deckhouse -o yaml
   ```

   Пример вывода:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleSource
   metadata:
     creationTimestamp: "2024-03-11T20:33:51Z"
     finalizers:
     - modules.deckhouse.io/release-exists
     generation: 1
     labels:
       heritage: deckhouse
     name: deckhouse
     resourceVersion: "20241841"
     uid: f35d10be-3ff9-4cd9-b64c-4f58abd8f595
   spec:
     registry:
       ca: |
         -----BEGIN CERTIFICATE-----
         ...
         -----END CERTIFICATE-----
       dockerCfg: ...
       repo: registry.example.com:5000/deckhouse/ee/modules
       scheme: HTTPS
     releaseChannel: ""
   status:
     message: ""
     moduleErrors: []
     modules:
     - name: deckhouse-admin
       policy: deckhouse
     - name: deckhouse-commander
       policy: deckhouse
     - name: deckhouse-commander-agent
       policy: deckhouse
     - name: operator-ceph
       policy: deckhouse
     - name: operator-postgres
       policy: deckhouse
     - name: sds-drbd
       policy: deckhouse
     - name: sds-node-configurator
       policy: deckhouse
     - name: secrets-store-integration
       policy: deckhouse
     - name: stronghold
       policy: deckhouse
     - name: virtualization
       policy: deckhouse
     modulesCount: 10
     syncTime: "2024-03-28T14:25:35Z"
   ```

Обратите внимание, что пустое значение для `spec.releaseChannel` говорит о том, что каналы обновлений для модулей будут совпадать с каналом обновлений для DKP.

5. Проверьте доступность новых выпусков для модулей, выполнив команду:

   ```bash
   kubectl get mr
   ```

   Пример вывода:

   ```yaml
   NAME                               PHASE        UPDATE POLICY   TRANSITIONTIME   MESSAGE
   deckhouse-admin-v1.19.3            Superseded                   91s              
   deckhouse-admin-v1.21.2            Deployed     deckhouse       91s              
   deckhouse-commander-agent-v1.0.1   Deployed                     16d              
   deckhouse-commander-v1.2.5         Deployed                     16d              
   operator-ceph-v1.0.10              Deployed                     16d              
   operator-postgres-v1.0.15          Deployed                     16d              
   sds-drbd-v0.1.7                    Deployed                     16d              
   sds-drbd-v0.1.8                    Pending      deckhouse       17m              Waiting for manual approval
   sds-node-configurator-v0.1.3       Deployed                     16d              
   sds-node-configurator-v0.1.7       Pending      deckhouse       17m              Waiting for manual approval
   sds-replicated-volume-v0.2.6       Pending      deckhouse       17m              Waiting for manual approval
   secrets-store-integration-v1.0.9   Deployed                     16d              
   stronghold-v1.0.9                  Deployed                     16d              
   virtualization-v0.9.10             Deployed                     16d 
   ```

Если модуль требует ручного подтверждения обновления, введите команду:

```bash
kubectl annotate mr sds-drbd-v0.1.8 modules.deckhouse.io/approved="true"
```

---
title: Выгрузка образов модулей DKP из репозитория вендора
permalink: ru/update/security/vendor-images/
lang: ru
---

Для выгрузки образов модулей DKP из репозитория вендора, сделайте следующие шаги:

1. Создайте зашифрованную base64 строку для доступа клиента Docker <!-- тут точно? У нас же нет Докера--> в репозиторий вендора. Сделать это можно, например, командой ниже, заменив `YOUR_USERNAME` на `license-token`, а `YOUR_PASSWORD` — на ваш лицензионный ключ:

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
