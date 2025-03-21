---
title: "Установка платформы"
permalink: ru/code/documentation/admin/install/install.html
lang: ru
---

## Установка и включение Deckhouse Code

1. **ModuleSource**. Создайте файл `code-module-source.yaml` со следующим содержимым:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleSource
   metadata:
     annotations:
       name: code
   spec:
     registry:
       ca: ""
       dockerCfg: REPLACE_ME
       repo: registry.flant.com/deckhouse/code/module
       scheme: HTTPS
   ```

   > `dockerCfg` — это закодированная в формате base64 часть конфигурационного файла docker-клиента (секция `auths`).

1. **ModuleUpdatePolicy**. Создайте файл `code-module-update-policy.yaml` со следующим содержимым:

   ```yaml
   apiVersion: deckhouse.io/v1alpha2
   kind: ModuleUpdatePolicy
   metadata:
     annotations:
       name: code-policy
   spec:
     releaseChannel: EarlyAccess
     update:
       mode: Auto
   ```

   > `releaseChannel` определяет канал обновлений (например, EarlyAccess).
   > `update.mode` указывает режим обновления (в данном примере — Auto).

1. **ModuleConfig**. Создайте файл `code-module-config.yaml` со следующим содержимым:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: code
   spec:
     enabled: true
     updatePolicy: code-policy
     version: 1
     settings:
       instanceSpec:
         gitData:
           storageClass: localpath
           storagePerReplicaGb: 1
         network:
           gitSsh:
             hostname: ""
             service:
               type: NodePort
         storages:
           s3:
             bucketNames:
               artifacts: <REPLACE_ME>-artifacts
               ciSecureFiles: <REPLACE_ME>-ci-secure-files
               dependencyProxy: <REPLACE_ME>-dependency-proxy
               externalDiffs: <REPLACE_ME>-mr-diffs
               lfs: <REPLACE_ME>-lfs
               packages: <REPLACE_ME>-packages
               terraformState: <REPLACE_ME>-terraform-state
               uploads: <REPLACE_ME>-uploads
             external:
               accessKey: <REPLACE_ME>
               provider: YCloud
               secretKey: <REPLACE_ME>
             mode: External
           postgres:
             external:
               database: app_db
               host: <REPLACE_ME>
               password: <REPLACE_ME>
               praefectDatabase: praefect_db
               praefectPassword: <REPLACE_ME>
               praefectUsername: code_user
               username: code_user
             mode: External
           redis:
             external:
               auth:
                 enabled: true
                 password: <REPLACE_ME>
               host: <REPLACE_ME>
               port: 6379
             mode: External
         targetUserCount: 10
   ```

   > `REPLACE_ME` — уникальные значения, требующие подстановки, такие как адреса зависимостей, имена пользователей и пароли.

1. **Включение модуля**. После создания всех манифестов поочерёдно примените их к целевому кластеру Kubernetes с помощью следующих команд:

   ```console
   kubectl apply -f code-module-source.yaml
   kubectl apply -f code-module-update-policy.yaml
   kubectl apply -f code-module-config.yaml
   ```
