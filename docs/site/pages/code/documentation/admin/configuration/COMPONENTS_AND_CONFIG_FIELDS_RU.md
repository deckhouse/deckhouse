---
title: "Компоненты и параметры конфигурации"
permalink: ru/code/documentation/admin/configuration/components.html
lang: ru
---

## Обзор компонентов Code

Cуществующие компоненты и их функции:

1. `Gitaly` — Git RPC-сервис для обработки всех Git-запросов, сделанных GitLab.
1. `raefect` — прозрачный прокси между любым Git-клиентом и узлами хранения Gitaly.
1. `Sidekiq` — процессор фоновых заданий.
1. `Webservice` — предоставляет пользовательский интерфейс и публичный API продукта.
1. `Webservice-internal-api` — предоставляет внутренний API для коммуникации компонентов между собой
1. `Shell` — программа, разработанная в GitLab для обработки Git-сессий на основе SSH и изменения списка авторизованных ключей.
1. `Toolbox` — многофункциональный инструмент, который позволяет администраторам восстанавливать данные из резервных копий или использовать rails-консоль.
1. `Exporter` — процесс, разработанный внутри компании, позволяющий экспортировать метрики о внутренней работе приложения Code в Prometheus.
1. `MRA` — означает утверждение слияния (merge request approval). Сервис, реализующий соответствующий функционал GitLab, включая возможность CODEOWNERS.
1. `Migrations-job` — задание, выполняющее миграции базы данных.
1. `Backup-cronjob` — cron-задача, отвечающая за процесс резервного копирования.
1. `Runner-controller` — Kubernetes-контроллер, управляющий конфигурацией и жизненным циклом GitLab Runner'ов внутри кластера. Опциональный компонент.
1. `Pages` — функция, позволяющая публиковать статические веб-сайты непосредственно из репозитория в GitLab. Опциональный компонент.
1. `Registry` — реестр контейнеров, который позволяет загружать и скачивать образы. Опциональный компонент.

Подробнее с компонентами можно познакомиться по [в официальной документации GitLab](https://docs.gitlab.com/ee/development/architecture.html).

## Все параметры ModuleConfig

> **Внимание**. Некоторые параметры могут быть взаимоисключающими (например, `redis.host` и `redis.sentinel`).

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: code
spec:
  enabled: true
  version: 1
  settings:
    appConfig:
      omniauth:
        enabled: <bool>
        allowBypassTwoFactor: <bool>
        allowSingleSignOn:  <bool|array>
        autoLinkLdapUser: <bool>
        autoLinkSamlUser: <bool>
        autoLinkUser: <bool>
        autoSignInWithProvider: <string>
        blockAutoCreatedUsers: <bool>
        syncProfileAttributes: <array>
        syncProfileFromProvider: <array>
        externalProviders: <array>
        providers: <array>
      rackAttack:
        gitlabBasicAuth:
          ipWhitelist: <array>
          maxretry: <int>
          findtime: <int>
          bantime: <int>
      signinEnabled: <bool>
      signupEnabled: <bool>
      usernameChangeEnabled: <bool>
    targetUserCount: <int>
    backup:
      restoreFromBackupMode: <bool>
      backupStorageGb: <int>
      enabled: <bool>
      s3:
        bucketName: <string>
        accessKey: <string> 
        provider: <string>
        region: <string>
        secretKey: <string>
      cronSchedule: <string>
      persistentVolumeClaim:
        enabled: <bool>
        storageClass: <string>
        size: <string>
    gitData:
      storagePerReplicaGb: <int>
      storageClass: <string>
    storages: 
      postgres:
        mode: <string>
        external:
          host: <string>
          port: <int>
          database: <string>
          username: <string>
          password: <string>
          praefectDatabase: <string>
          praefectUsername: <string>
          praefectPassword: <string>
        intenal: {}
      redis:
        mode: <string>
        external:
          auth:
            enabled: <bool>
            passowrd: <string>
          host: <string>
          port: <string>
          sentinels: <array>
          masterName: <string>
        internal: {}
      s3:
        mode: <string>
        bucketNames:
          artifacts: <string>
          ciSecureFiles: <string>
          dependecyProxy: <string>
          externalDiffs: <string>
          lfs: <string>
          packages: <string>
          terraformState: <string>
          uploads: <string>
        external:
          provider: <string>
          region: <string>
          endpoint: <string>
          accessKey: <string>
          secretKey: <string>
          storageOptions:
            serverSideEncryption: <string>
            serverSideEncryptionKmsKeyId: <string>
        internal: {}
    network:
      web:
        hostname: <string>
        annotations: {}
        https:
          mode: <string>
          customCertificate:
            secretName: <string>
          certManager:
            clusterIssuerName: <string>
      gitSsh:
        hostname: <string>
        service:
          annotations: {}
          type: <string>
          nodePort: <int>
    features:
      mail:
        outgoingEmail:
          displayName: <string>
          from: <string>
          replyTo: <string>
          subjectSuffix: <string>
          smtp:
            enabled: <bool>
            address: <string>
            port: <int>
            authentication: <string>
            username: <string>
            password: <string>
            domain: <string>
            tls: <bool>
            starttlsAuto: <bool>
            opensslVerifyMode: <string>
        incomingEmail:
          enabled: <bool>
          address: <string>
          host: <string>
          port: <int>
          ssl: <bool>
          startTls: <bool>
          user: <string>
          password: <string>
          serviceDeskEmail:
            enabled: <bool>
            address: <string>
            host: <string>
            port: <int>
            ssl: <bool>
            startTls: <bool>
            user: <string>
            password: <string>
      pages:
        enabled: <bool>
        s3:
          mode: <string>
          bucketPrefix: <string>
          internal: {}
          external:
            provider: <string>
            region: <string>
            endpoint: <string>
            accessKey: <string>
            secretKey: <string>
            storageOptions:
              serverSideEncryption: <string>
              serverSideEncryptionKmsKeyId: <string>
      registry:
        enabled: <bool>
        s3:
          mode: <string>
          bucketName: <string>
          internal: {}
          external:
            provider: <string>
            region: <string>
            endpoint: <string>
            accessKey: <string>
            secretKey: <string>
        ingress:
          https:
            mode: <string>
            customCertificate:
              secretName: <string>
            certManager:
              clusterIssuerName: <string>
        maintenance:
          readOnly: <bool>
          uploadPuring:
            enabled: <bool>
            age: <string>
            interval: <string>
            dryrun: <bool>
      runnerController:
        enabled: <bool>
```

## Подробное описание параметров ModuleConfig

Значение параметров:

- `enabled` — включает или отключает модуль
- `settings` — общий раздел, который инкапсулирует все низкоуровневые параметры конфигурации модуля
- `settings.targetUserCount` — сколько пользователей предполагается использовать приложение. Это влияет на множество внутренних аспектов, отвечающих за горизонтальное и вертикальное масштабирование приложения. Подробнее [в документации](../../configuration/scaling.html)
- `settings.appConfig` — конфигурация приложения Code. Семантически идентична `gitlab.rb` в GitLab для облегчения переноса настроек приложения из существующего экземпляра
  - `settings.appConfig.omniauth` — раздел с настройками Omniauth. Подробнее [в документации](https://docs.gitlab.com/ee/integration/omniauth.html#configure-common-settings)
    - `settings.appConfig.omniauth.enabled` — включено ли Omniauth
    - `settings.appConfig.omniauth.allowBypassTwoFactor` — разрешить обход двухфакторной аутентификации
    - `settings.appConfig.omniauth.allowSingleSignOn` — разрешить единый вход (SSO)
    - `settings.appConfig.omniauth.autoLinkSamlUser` — автоматически связывать пользователей SAML
    - `settings.appConfig.omniauth.autoLinkUser` — автоматически связывать пользователя
    - `settings.appConfig.omniauth.autoSignInWithProvider` — автоматический вход через провайдера
    - `settings.appConfig.omniauth.blockAutoCreatedUsers` — блокировать автоматически созданных пользователей
    - `settings.appConfig.omniauth.syncProfileAttributes` — синхронизировать атрибуты профиля
    - `settings.appConfig.omniauth.syncProfileFromProvider` — синхронизировать профиль с провайдера
    - `settings.appConfig.omniauth.externalProviders` — внешние провайдеры
    - `settings.appConfig.omniauth.providers` — провайдеры Omniauth
  - `settings.appConfig.rackAttack.gitlabBasicAuth` — ограничитель скорости, используемый GitLab для настройки блокировки и ограничения пользователей. Подробнее [в документации](https://gitlab.com/gitlab-org/gitlab-foss/-/blob/237bddc6a52fdc8ccb51b024a3048a3233ee43a3/doc/security/rack_attack.md)
    - `settings.appConfig.rackAttack.gitlabBasicAuth.ipWhitelist` — список IP-адресов, не подпадающих под правила rackAttack
    - `settings.appConfig.rackAttack.gitlabBasicAuth.maxretry` — ограничение числа попыток аутентификации Git HTTP на IP
    - `settings.appConfig.rackAttack.gitlabBasicAuth.findtime` — сбросить счетчик попыток аутентификации на IP через N секунд
    - `settings.appConfig.rackAttack.gitlabBasicAuth.bantime` — заблокировать IP на N секунд после слишком большого количества попыток
  - `settings.appConfig.signinEnabled` — включить ли страницу входа
  - `settings.appConfig.signupEnabled` — разрешить ли регистрацию новых пользователей
  - `settings.appConfig.usernameChangeEnabled` — разрешить изменение имен пользователей для существующих учетных записей
  - `settings.appConfig.defaultColorMode` — тема по умолчанию для интерфейса
- `settings.backup` — раздел, связанный с процессом резервного копирования продукта
  - `settings.backup.enabled` — включен ли процесс резервного копирования
  - `settings.backup.restoreFromBackupMode` — специальный режим приложения, используемый во время резервного копирования/восстановления. Нужен для отключения всех потенциальных потребителей приложения и предотвращения неконсистентности процессов
  - `settings.backup.cronSchedule` — расписание в формате cron для выполнения операций резервного копирования
  - `settings.backup.backupStorageGb` — ожидаемый общий размер резервных копий (архив tar) для настройки базового хранилища
  - `settings.backup.s3` — раздел для описания параметров удаленного объектного хранилища, которое будет содержать резервные копии
    - `settings.backup.s3.bucketName` — имя бакета в объектном хранилище
    - `settings.backup.s3.provider` — провайдер объектного хранилища
    - `settings.backup.s3.region` — регион бакета
    - `settings.backup.s3.accessKey` — ключ доступа для бакета
    - `settings.backup.s3.secretKey` — секретный ключ для бакета
  - `settings.backup.persistentVolumeClaim` — раздел для описания параметров постоянного хранилища Kubernetes, используемого во время резервного копирования и восстановления
    - `settings.backup.persistentVolumeClaim.enabled` — использовать ли постоянные тома. В противном случае будет использоваться `emptyDir`
    - `settings.backup.persistentVolumeClaim.storageClass` — использовать ли конкретный StorageClass Kubernetes для постоянных томов
- `settings.gitData` — все, что связано с вашими данными Git
  - `settings.gitData.storageClass` — StorageClass Kubernetes для использования с постоянными томами
  - `settings.gitData.storagePerReplicaGb` — размер всех данных Git. Необходим для расчета размера томов для каждой реплики
- `settings.storages` — все, что связано с хранилищами, используемыми `Code`. В настоящее время поддерживаются только внешние хранилища
  - `settings.storages.postgres` — раздел с настройками базы данных PostgreSQL
    - `settings.storages.postgres.mode` — использовать внешнюю или внутреннюю базу данных
    - `settings.storages.postgres.external` — раздел с настройками внешней базы данных PostgreSQL
      - `settings.storages.postgres.external.host` — FQDN-адрес сервера базы данных
      - `settings.storages.postgres.external.port` — порт базы данных PostgreSQL
      - `settings.storages.postgres.external.database` — основное имя базы данных
      - `settings.storages.postgres.external.username` — имя пользователя для основной базы данных
      - `settings.storages.postgres.external.password` — пароль для основной базы данных
      - `settings.storages.postgres.external.praefectDatabase` — имя базы данных для Praefect
      - `settings.storages.postgres.external.praefectUsername` — имя пользователя для базы данных Praefect
      - `settings.storages.postgres.external.praefectPassword` — пароль для базы данных Praefect
    - `settings.storages.postgres.internal` — в настоящее время не поддерживается. Раздел для будущего использования
  - `settings.storages.redis` — раздел с настройками брокера сообщений Redis
    - `settings.storages.redis.mode` — будет использоваться внешний или встроенный брокер сообщений
    - `settings.storages.redis.external` — раздел с настройками внешнего брокера сообщений Redis
      - `settings.storages.redis.external.auth` — раздел для аутентификации Redis
        - `settings.storages.redis.external.auth.enabled` — включена ли аутентификация для внешнего Redis
        - `settings.storages.redis.external.auth.password` — пароль для Redis, если аутентификация включена
      - `settings.storages.redis.external.host` — FQDN для единственного экземпляра Redis. Не требуется, если используются sentinels
      - `settings.storages.redis.external.port` — порт для единственного экземпляра Redis. Не требуется, если используются sentinels
      - `settings.storages.redis.external.sentinels` — раздел с массивом для карт хостов и портов sentinels. Необходим только при использовании sentinels
        - `settings.storages.redis.external.sentinels.host` — хост конкретного экземпляра sentinel
        - `settings.storages.redis.external.sentinels.port` — порт конкретного экземпляра sentinel
      - `settings.storages.redis.external.masterName` — имя мастер-узла. Используется только с sentinels
    - `settings.storages.redis.internal` — в настоящее время не поддерживается. Раздел для будущего использования
  - `settings.storages.s3` — раздел с настройками объектного хранилища
    - `settings.storages.s3.mode` — будет использоваться внешнее или встроенное объектное хранилище
    - `settings.storages.s3.bucketNames` — мапа (ключ-значение), указывающая имена для всех необходимых бакетов
      - `settings.storages.s3.bucketNames.artifacts`
      - `settings.storages.s3.bucketNames.ciSecureFiles`
      - `settings.storages.s3.bucketNames.dependencyProxy`
      - `settings.storages.s3.bucketNames.externalDiffs`
      - `settings.storages.s3.bucketNames.lfs`
      - `settings.storages.s3.bucketNames.packages`
      - `settings.storages.s3.bucketNames.terraformState`
      - `settings.storages.s3.bucketNames.uploads`
    - `settings.storages.s3.external` — раздел с параметрами для внешнего объектного хранилища
      - `settings.storages.s3.external.provider` — провайдер объектного хранилища
      - `settings.storages.s3.external.region` — регион бакетов
      - `settings.storages.s3.external.endpoint` — endpoint бакетов
      - `settings.storages.s3.external.accessKey` — ключ доступа к бакетам
      - `settings.storages.s3.external.secretKey` — секретный ключ для бакетов
      - `settings.storages.s3.external.storageOptions` — раздел с низкоуровневыми настройками шифрования для объектного хранилища компонента
        - `settings.storages.s3.external.storageOptions.serverSideEncryption` — режим шифрования (AES256 или aws:kms)
        - `settings.storages.s3.external.storageOptions.serverSideEncryptionKmsKeyId` — Amazon Resource Name. Необходим только при использовании aws:kms для serverSideEncryption
    - `settings.storages.s3.internal` — в настоящее время не поддерживается. Раздел для будущего использования
- `settings.network` — раздел для настройки параметров сети
  - `settings.network.web` — раздел для настройки сети веб-сервиса (UI)
    - `settings.network.web.hostname` — верхний префикс для имени хоста UI
    - `settings.network.web.annotations` — дополнительные аннотации (в формате ключ-значение) для развертывания веб-сервиса
    - `settings.network.web.https` — раздел с настройками https, в частности с параметрами, связанными с сертификатами
      - `settings.network.web.https.mode` — способ хранения/выдачи сертификата. Может быть custom/certManager/global
        - `settings.network.web.https.customCertificate.secretName` — имя секрета, где хранится пользовательский сертификат
        - `settings.network.web.https.certManager.clusterIssuerName` — имя кластерного issuer для SSL-сертификатов
  - `settings.network.gitSsh` — раздел для настройки сети компонента shell (для поддержки git по SSH)
    - `settings.network.gitSsh.hostname`— переопределение имени хоста shell для отличия от стандартного
    - `settings.network.gitSsh.annotations` — дополнительные аннотации (в формате ключ-значение) для развертывания shell
    - `settings.network.gitSsh.service` — раздел с настройками Kubernetes-сервиса для компонента shell
      - `settings.network.gitSsh.service.type` — тип Kubernetes-сервиса для экспонирования компонента shell. Для одноузлового кластера подходит nodePort, для остальных рекомендуется LoadBalancer
      - `settings.network.gitSsh.service.nodePort` — nodePort для экспонирования сервиса. Применимо только для service.type=NodePort
- `settings.features` — другие необязательные компоненты, которые включаются по требованию. Вся конфигурация ниже относится к компонентам
  - `settings.features.runnerController` — раздел для включения и настройки компонента runner-controller
    - `settings.features.runnerController.enabled` — включать ли развертывание компонента
  - `settings.features.mail` — раздел с настройками для различных типов почты: входящей, исходящей, serviceDesk
    - `settings.features.mail.outgoingEmail` — раздел с настройками для исходящей почты. Подробнее о параметрах можно прочитать [в документации](https://docs.gitlab.com/charts/installation/command-line-options.html#outgoing-email-configuration)
      - `settings.features.mail.outgoingEmail.displayName`
      - `settings.features.mail.outgoingEmail.from`
      - `settings.features.mail.outgoingEmail.replyTo`
      - `settings.features.mail.outgoingEmail.subjectSuffix`
      - `settings.features.mail.outgoingEmail.smtp`
        - `settings.features.mail.outgoingEmail.smtp.enabled`
        - `settings.features.mail.outgoingEmail.smtp.address`
        - `settings.features.mail.outgoingEmail.smtp.port`
        - `settings.features.mail.outgoingEmail.smtp.authentication`
        - `settings.features.mail.outgoingEmail.smtp.username`
        - `settings.features.mail.outgoingEmail.smtp.password`
        - `settings.features.mail.outgoingEmail.smtp.domain`
        - `settings.features.mail.outgoingEmail.smtp.tls`
        - `settings.features.mail.outgoingEmail.smtp.starttlsAuto`
        - `settings.features.mail.outgoingEmail.smtp.opensslVerifyMode`
    - `settings.features.mail.incomingEmail` — раздел с настройками для входящей почты. Подробнее о параметрах можно прочитать [в документации](https://docs.gitlab.com/charts/installation/command-line-options.html#incoming-email-configuration)
      - `settings.features.mail.incomingEmail.enabled`
      - `settings.features.mail.incomingEmail.address`
      - `settings.features.mail.incomingEmail.host`
      - `settings.features.mail.incomingEmail.port`
      - `settings.features.mail.incomingEmail.ssl`
      - `settings.features.mail.incomingEmail.startTls`
      - `settings.features.mail.incomingEmail.user`
      - `settings.features.mail.incomingEmail.password`
      - `settings.features.mail.serviceDeskEmail` — раздел с настройками для почты службы поддержки. Подробнее о параметрах можно прочитать [в документации](https://docs.gitlab.com/charts/installation/command-line-options.html#service-desk-email-configuration)
        - `settings.features.mail.incomingEmail.enabled`
        - `settings.features.mail.incomingEmail.address`
        - `settings.features.mail.incomingEmail.host`
        - `settings.features.mail.incomingEmail.port`
        - `settings.features.mail.incomingEmail.ssl`
        - `settings.features.mail.incomingEmail.startTls`
        - `settings.features.mail.incomingEmail.user`
        - `settings.features.mail.incomingEmail.password`
  - `settings.features.pages` — раздел с настройками компонента Pages
    - `settings.features.pages.enabled` — включить ли компонент
    - `settings.features.pages.s3` — раздел с настройками объектного хранилища для компонента
      - `settings.features.pages.s3.mode` — использовать внешнее или встроенное объектное хранилище
      - `settings.features.pages.s3.bukcetPrefix` — префикс для имени бакетов. Финальные имена будут иметь вид {prefix}-{name}
      - `settings.features.pages.s3.internal` — в настоящее время не поддерживается. Для будущего использования
      - `settings.features.pages.s3.external` — раздел с настройками внешнего объектного хранилища
        - `settings.features.pages.s3.external.endpoint` — пользовательский endpoint для объектного хранилища
        - `settings.features.pages.s3.external.provider` — провайдер объектного хранилища
        - `settings.features.pages.s3.external.region` — регион бакетов
        - `settings.features.pages.s3.external.accessKey` — ключ доступа к бакету
        - `settings.features.pages.s3.external.secretKey` — секретный ключ для бакета
        - `settings.features.pages.s3.external.azureAccountName` — имя учетной записи AzureRM
        - `settings.features.pages.s3.external.azureAccessKey` — ключ доступа к объектному хранилищу AzureRM
        - `settings.features.pages.s3.external.storageOptions` — раздел с низкоуровневыми настройками шифрования для объектного хранилища компонента
          - `settings.features.pages.s3.external.storageOptions.serverSideEncryption` — режим шифрования (AES256 или aws:kms)
          - `settings.features.pages.s3.external.storageOptions.serverSideEncryptionKmsKeyId` — Amazon Resource Name. Необходим только при использовании aws:kms для serverSideEncryption
  - `settings.features.registry` — раздел с настройками компонента Registry
    - `settings.features.registry.enabled` — включить ли компонент
    - `settings.features.registry.ingress` — раздел с настройками входящих сетевых соединений для компонента
      - `settings.features.registry.ingress.https` — настройки https для входящих соединений компонента
        - `settings.features.registry.ingress.https.mode` — способ хранения/выдачи сертификата. Может быть custom/certManager/global
        - `settings.features.registry.ingress.https.certManager.clusterIssuerName` — имя кластерного issuer для SSL-сертификатов
        - `settings.features.registry.ingress.https.customCertificate.secretName` — имя секрета, где хранится пользовательский сертификат
    - `settings.features.registry.s3` — раздел с настройками объектного хранилища для компонента
      - `settings.features.registry.s3.mode` — использовать внешнее или встроенное объектное хранилище для компонента Registry
      - `settings.features.registry.s3.bucketName` — имя бакета для компонента Registry
      - `settings.features.registry.s3.external` — раздел с настройками внешнего объектного хранилища
        - `settings.features.registry.s3.external.provider` — провайдер объектного хранилища
        - `settings.features.registry.s3.external.endpoint` — пользовательский endpoint для объектного хранилища
        - `settings.features.registry.s3.external.region` — регион бакетов
        - `settings.features.registry.s3.external.accessKey` — ключ доступа к бакету
        - `settings.features.registry.s3.external.secretKey` — секретный ключ для бакета
        - `settings.features.registry.s3.external.azureAccountName` — имя учетной записи AzureRM
        - `settings.features.registry.s3.external.azureAccessKey` — ключ доступа к объектному хранилищу AzureRM
      - `settings.features.registry.s3.internal` — в настоящее время не поддерживается. Для будущего использования
      - `settings.features.registry.s3.bucketName` — имя бакета, который будет использоваться как объектное хранилище для Registry
    - `settings.features.registry.maintenance` — раздел с настройками обслуживания Registry
      - `settings.features.registry.maintenance.readOnly` — перевести ли Registry в режим только для чтения во время обслуживания
      - `settings.features.registry.maintenance.uploadPurging` — раздел описывает функцию обслуживания, удаляющую артефакты загрузки изображений из хранилища (данные изображений не удаляются)
        - `settings.features.registry.maintenance.uploadPurging.enabled` — включить ли функцию uploadPurging. Выключено в режиме readOnly
        - `settings.features.registry.maintenance.uploadPurging.age` — возраст артефактов для удаления, измеряется в часах
        - `settings.features.registry.maintenance.uploadPurging.interval` — интервал запуска
        - `settings.features.registry.maintenance.uploadPurging.dryrun` — использовать ли в тестовом/проверочном режиме
