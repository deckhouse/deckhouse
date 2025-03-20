---
title: "Компоненты и параметры конфигурации"
permalink: ru/code/documentation/admin/configuration/components.html
lang: ru
---

## Обзор компонентов Code

Cуществующие компоненты и их функции:

1. `Gitaly` — Git RPC-сервис для обработки всех Git-запросов, сделанных Deckhouse Code.
1. `raefect` — прозрачный прокси между любым Git-клиентом и узлами хранения Gitaly.
1. `Sidekiq` — процессор фоновых заданий.
1. `Webservice` — предоставляет пользовательский интерфейс и публичный API продукта.
1. `Webservice-internal-api` — предоставляет внутренний API для коммуникации компонентов между собой
1. `Shell` — программа для обработки Git-сессий на основе SSH и изменения списка авторизованных ключей.
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
    logLevel: <string>
    instanceSpec:
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
- `settings.logLevel` - уровень логирования для оператора, занимающегося установкой и обслуживанием компонентов Deckhouse Code
- `settings.instanceSpec` - секция, содержащая все настройки инсталляции Deckhouse Code
  - `settings.instanceSpec.targetUserCount` - сколько пользователей предполагается использовать приложение. Это влияет на множество внутренних аспектов, отвечающих за горизонтальное и вертикальное масштабирование приложения. Подробнее [здесь](./SCALING.md)
  - `settings.instanceSpec.appConfig` - конфигурация приложения Code. Семантически идентична `gitlab.rb` в GitLab для облегчения переноса настроек приложения из существующего экземпляра
    - `settings.instanceSpec.appConfig.omniauth` - раздел с настройками Omniauth. Подробнее [здесь](https://docs.gitlab.com/ee/integration/omniauth.html#configure-common-settings)
      - `settings.instanceSpec.appConfig.omniauth.enabled` - включено ли Omniauth
      - `settings.instanceSpec.appConfig.omniauth.allowBypassTwoFactor` - разрешить обход двухфакторной аутентификации
      - `settings.instanceSpec.appConfig.omniauth.allowSingleSignOn` - разрешить единый вход (SSO)
      - `settings.instanceSpec.appConfig.omniauth.autoLinkSamlUser` - автоматически связывать пользователей SAML
      - `settings.instanceSpec.appConfig.omniauth.autoLinkUser` - автоматически связывать пользователя
      - `settings.instanceSpec.appConfig.omniauth.autoSignInWithProvider` - автоматический вход через провайдера
      - `settings.instanceSpec.appConfig.omniauth.blockAutoCreatedUsers` - блокировать автоматически созданных пользователей
      - `settings.instanceSpec.appConfig.omniauth.syncProfileAttributes` - синхронизировать атрибуты профиля
      - `settings.instanceSpec.appConfig.omniauth.syncProfileFromProvider` - синхронизировать профиль с провайдера
      - `settings.instanceSpec.appConfig.omniauth.externalProviders` - внешние провайдеры
      - `settings.instanceSpec.appConfig.omniauth.providers` - провайдеры Omniauth
    - `settings.instanceSpec.appConfig.rackAttack.gitlabBasicAuth` - ограничитель скорости, используемый GitLab для настройки блокировки и ограничения пользователей. Подробнее [здесь](https://gitlab.com/gitlab-org/gitlab-foss/-/blob/237bddc6a52fdc8ccb51b024a3048a3233ee43a3/doc/security/rack_attack.md)
      - `settings.instanceSpec.appConfig.rackAttack.gitlabBasicAuth.ipWhitelist` - список IP-адресов, не подпадающих под правила rackAttack
      - `settings.instanceSpec.appConfig.rackAttack.gitlabBasicAuth.maxretry` - ограничение числа попыток аутентификации Git HTTP на IP
      - `settings.instanceSpec.appConfig.rackAttack.gitlabBasicAuth.findtime` - сбросить счетчик попыток аутентификации на IP через N секунд
      - `settings.instanceSpec.appConfig.rackAttack.gitlabBasicAuth.bantime` - заблокировать IP на N секунд после слишком большого количества попыток
    - `settings.instanceSpec.appConfig.signinEnabled` - включить ли страницу входа
    - `settings.instanceSpec.appConfig.signupEnabled` - разрешить ли регистрацию новых пользователей
    - `settings.instanceSpec.appConfig.usernameChangeEnabled` - разрешить изменение имен пользователей для существующих учетных записей
    - `settings.instanceSpec.appConfig.defaultColorMode` - тема по умолчанию для интерфейса
  - `settings.instanceSpec.backup` - раздел, связанный с процессом резервного копирования продукта
    - `settings.instanceSpec.backup.enabled` - включен ли процесс резервного копирования
    - `settings.instanceSpec.backup.restoreFromBackupMode` - специальный режим приложения, используемый во время резервного копирования/восстановления. Нужен для отключения всех потенциальных потребителей приложения и предотвращения неконсистентности процессов
    - `settings.instanceSpec.backup.cronSchedule` - расписание в формате cron для выполнения операций резервного копирования
    - `settings.instanceSpec.backup.backupStorageGb` - ожидаемый общий размер резервных копий (архив tar) для настройки базового хранилища
    - `settings.instanceSpec.backup.s3` - раздел для описания параметров удаленного объектного хранилища, которое будет содержать резервные копии
      - `settings.instanceSpec.backup.s3.bucketName` - имя бакета в объектном хранилище
      - `settings.instanceSpec.backup.s3.provider` - провайдер объектного хранилища
      - `settings.instanceSpec.backup.s3.region` - регион бакета
      - `settings.instanceSpec.backup.s3.accessKey` - ключ доступа для бакета
      - `settings.instanceSpec.backup.s3.secretKey` - секретный ключ для бакета
    - `settings.instanceSpec.backup.persistentVolumeClaim` - раздел для описания параметров постоянного хранилища Kubernetes, используемого во время резервного копирования и восстановления
      - `settings.instanceSpec.backup.persistentVolumeClaim.enabled` - использовать ли постоянные тома. В противном случае будет использоваться `emptyDir`
      - `settings.instanceSpec.backup.persistentVolumeClaim.storageClass` - использовать ли конкретный storageClass Kubernetes для постоянных томов
  - `settings.instanceSpec.gitData` - все, что связано с вашими данными Git
    - `settings.instanceSpec.gitData.storageClass` - storageClass Kubernetes для использования с постоянными томами
    - `settings.instanceSpec.gitData.storagePerReplicaGb` - размер всех данных Git. Необходим для расчета размера томов для каждой реплики
    - `settings.instanceSpec.gitData.resources` - секция определяет размер ресурсов на ресурсов, обслуживающих данные Git (gitaly)
      - `settings.instanceSpec.gitData.resources.memory` - выделенное количество RAM
      - `settings.instanceSpec.gitData.resources.cpu` - выделенное количество CPU
  - `settings.instanceSpec.storages` - все, что связано с хранилищами, используемыми `Code`. В настоящее время поддерживаются только внешние хранилища
    - `settings.instanceSpec.storages.postgres` - раздел с настройками базы данных PostgreSQL
      - `settings.instanceSpec.storages.postgres.mode` - использовать внешнюю или внутреннюю базу данных
      - `settings.instanceSpec.storages.postgres.external` - раздел с настройками внешней базы данных PostgreSQL
        - `settings.instanceSpec.storages.postgres.external.host` - FQDN-адрес сервера базы данных
        - `settings.instanceSpec.storages.postgres.external.port` - порт базы данных PostgreSQL
        - `settings.instanceSpec.storages.postgres.external.database` - основное имя базы данных
        - `settings.instanceSpec.storages.postgres.external.username` - имя пользователя для основной базы данных
        - `settings.instanceSpec.storages.postgres.external.password` - пароль для основной базы данных
        - `settings.instanceSpec.storages.postgres.external.praefectDatabase` - имя базы данных для Praefect
        - `settings.instanceSpec.storages.postgres.external.praefectUsername` - имя пользователя для базы данных Praefect
        - `settings.instanceSpec.storages.postgres.external.praefectPassword` - пароль для базы данных Praefect
      - `settings.instanceSpec.storages.postgres.internal` - в настоящее время не поддерживается. Раздел для будущего использования
    - `settings.instanceSpec.storages.redis` - раздел с настройками брокера сообщений Redis
      - `settings.instanceSpec.storages.redis.mode` - будет использоваться внешний или встроенный брокер сообщений
      - `settings.instanceSpec.storages.redis.external` - раздел с настройками внешнего брокера сообщений Redis
        - `settings.instanceSpec.storages.redis.external.auth` - раздел для аутентификации Redis
          - `settings.instanceSpec.storages.redis.external.auth.enabled` - включена ли аутентификация для внешнего Redis
          - `settings.instanceSpec.storages.redis.external.auth.password` - пароль для Redis, если аутентификация включена
        - `settings.instanceSpec.storages.redis.external.host` - FQDN для единственного экземпляра Redis. Не требуется, если используются sentinels
        - `settings.instanceSpec.storages.redis.external.port` - порт для единственного экземпляра Redis. Не требуется, если используются sentinels
        - `settings.instanceSpec.storages.redis.external.sentinels` - раздел с массивом для карт хостов и портов sentinels. Необходим только при использовании sentinels
          - `settings.instanceSpec.storages.redis.external.sentinels.host` - хост конкретного экземпляра sentinel
          - `settings.instanceSpec.storages.redis.external.sentinels.port` - порт конкретного экземпляра sentinel
        - `settings.instanceSpec.storages.redis.external.masterName` - имя мастер-узла. Используется только с sentinels
      - `settings.instanceSpec.storages.redis.internal` - в настоящее время не поддерживается. Раздел для будущего использования
    - `settings.instanceSpec.storages.s3` - раздел с настройками объектного хранилища
      - `settings.instanceSpec.storages.s3.mode` - будет использоваться внешнее или встроенное объектное хранилище
      - `settings.instanceSpec.storages.s3.bucketNames` - мапа (ключ-значение), указывающая имена для всех необходимых бакетов
        - `settings.instanceSpec.storages.s3.bucketNames.artifacts`
        - `settings.instanceSpec.storages.s3.bucketNames.ciSecureFiles`
        - `settings.instanceSpec.storages.s3.bucketNames.dependencyProxy`
        - `settings.instanceSpec.storages.s3.bucketNames.externalDiffs`
        - `settings.instanceSpec.storages.s3.bucketNames.lfs`
        - `settings.instanceSpec.storages.s3.bucketNames.packages`
        - `settings.instanceSpec.storages.s3.bucketNames.terraformState`
        - `settings.instanceSpec.storages.s3.bucketNames.uploads`
      - `settings.instanceSpec.storages.s3.external` - раздел с параметрами для внешнего объектного хранилища
        - `settings.instanceSpec.storages.s3.external.provider` - провайдер объектного хранилища
        - `settings.instanceSpec.storages.s3.external.region` - регион бакетов
        - `settings.instanceSpec.storages.s3.external.endpoint` - endpoint бакетов
        - `settings.instanceSpec.storages.s3.external.accessKey` - ключ доступа к бакетам
        - `settings.instanceSpec.storages.s3.external.secretKey` - секретный ключ для бакетов
        - `settings.instanceSpec.storages.s3.external.storageOptions` - раздел с низкоуровневыми настройками шифрования для объектного хранилища компонента
          - `settings.instanceSpec.storages.s3.external.storageOptions.serverSideEncryption` - режим шифрования (AES256 или aws:kms)
          - `settings.instanceSpec.storages.s3.external.storageOptions.serverSideEncryptionKmsKeyId` - Amazon Resource Name. Необходим только при использовании aws:kms для serverSideEncryption
      - `settings.instanceSpec.storages.s3.internal` - в настоящее время не поддерживается. Раздел для будущего использования
  - `settings.instanceSpec.network` - раздел для настройки параметров сети
    - `settings.instanceSpec.network.web` - раздел для настройки сети веб-сервиса (UI)
      - `settings.instanceSpec.network.web.hostname` - верхний префикс для имени хоста UI
      - `settings.instanceSpec.network.web.annotations` - дополнительные аннотации (в формате ключ-значение) для развертывания веб-сервиса
      - `settings.instanceSpec.network.web.https` - раздел с настройками https, в частности с параметрами, связанными с сертификатами
        - `settings.instanceSpec.network.web.https.mode` - способ хранения/выдачи сертификата. Может быть custom/certManager/global
          - `settings.instanceSpec.network.web.https.customCertificate.secretName` - имя секрета, где хранится пользовательский сертификат
          - `settings.instanceSpec.network.web.https.certManager.clusterIssuerName` - имя кластерного issuer для SSL-сертификатов
    - `settings.instanceSpec.network.gitSsh` - раздел для настройки сети компонента shell (для поддержки git по SSH)
      - `settings.instanceSpec.network.gitSsh.hostname`- переопределение имени хоста shell для отличия от стандартного
      - `settings.instanceSpec.network.gitSsh.annotations` - дополнительные аннотации (в формате ключ-значение) для развертывания shell
      - `settings.instanceSpec.network.gitSsh.service` - раздел с настройками kubernetes-сервиса для компонента shell
        - `settings.instanceSpec.network.gitSsh.service.type` - тип kubernetes-сервиса для экспонирования компонента shell. Для одноузлового кластера подходит nodePort, для остальных рекомендуется LoadBalancer
        - `settings.instanceSpec.network.gitSsh.service.nodePort` - nodePort для экспонирования сервиса. Применимо только для service.type=NodePort
  - `settings.instanceSpec.features` - другие необязательные компоненты, которые включаются по требованию. Вся конфигурация ниже относится к компонентам
    - `settings.instanceSpec.features.runnerController` - раздел для включения и настройки компонента runner-controller
      - `settings.instanceSpec.features.runnerController.enabled` - включать ли развертывание компонента
    - `settings.instanceSpec.features.mail` - раздел с настройками для различных типов почты: входящей, исходящей, serviceDesk
      - `settings.instanceSpec.features.mail.outgoingEmail` - раздел с настройками для исходящей почты. Подробнее о параметрах можно прочитать [здесь](https://docs.gitlab.com/charts/installation/command-line-options.html#outgoing-email-configuration)
        - `settings.instanceSpec.features.mail.outgoingEmail.displayName`
        - `settings.instanceSpec.features.mail.outgoingEmail.from`
        - `settings.instanceSpec.features.mail.outgoingEmail.replyTo`
        - `settings.instanceSpec.features.mail.outgoingEmail.subjectSuffix`
        - `settings.instanceSpec.features.mail.outgoingEmail.smtp`
          - `settings.instanceSpec.features.mail.outgoingEmail.smtp.enabled`
          - `settings.instanceSpec.features.mail.outgoingEmail.smtp.address`
          - `settings.instanceSpec.features.mail.outgoingEmail.smtp.port`
          - `settings.instanceSpec.features.mail.outgoingEmail.smtp.authentication`
          - `settings.instanceSpec.features.mail.outgoingEmail.smtp.username`
          - `settings.instanceSpec.features.mail.outgoingEmail.smtp.password`
          - `settings.instanceSpec.features.mail.outgoingEmail.smtp.domain`
          - `settings.instanceSpec.features.mail.outgoingEmail.smtp.tls`
          - `settings.instanceSpec.features.mail.outgoingEmail.smtp.starttlsAuto`
          - `settings.instanceSpec.features.mail.outgoingEmail.smtp.opensslVerifyMode`
      - `settings.instanceSpec.features.mail.incomingEmail` - раздел с настройками для входящей почты. Подробнее о параметрах можно прочитать [здесь](https://docs.gitlab.com/charts/installation/command-line-options.html#incoming-email-configuration)
        - `settings.instanceSpec.features.mail.incomingEmail.enabled`
        - `settings.instanceSpec.features.mail.incomingEmail.address`
        - `settings.instanceSpec.features.mail.incomingEmail.host`
        - `settings.instanceSpec.features.mail.incomingEmail.port`
        - `settings.instanceSpec.features.mail.incomingEmail.ssl`
        - `settings.instanceSpec.features.mail.incomingEmail.startTls`
        - `settings.instanceSpec.features.mail.incomingEmail.user`
        - `settings.instanceSpec.features.mail.incomingEmail.password`
        - `settings.instanceSpec.features.mail.serviceDeskEmail` - раздел с настройками для почты службы поддержки. Подробнее о параметрах можно прочитать [здесь](https://docs.gitlab.com/charts/installation/command-line-options.html#service-desk-email-configuration)
          - `settings.instanceSpec.features.mail.incomingEmail.enabled`
          - `settings.instanceSpec.features.mail.incomingEmail.address`
          - `settings.instanceSpec.features.mail.incomingEmail.host`
          - `settings.instanceSpec.features.mail.incomingEmail.port`
          - `settings.instanceSpec.features.mail.incomingEmail.ssl`
          - `settings.instanceSpec.features.mail.incomingEmail.startTls`
          - `settings.instanceSpec.features.mail.incomingEmail.user`
          - `settings.instanceSpec.features.mail.incomingEmail.password`
    - `settings.instanceSpec.features.pages` - раздел с настройками компонента Pages
      - `settings.instanceSpec.features.pages.enabled` - включить ли компонент
      - `settings.instanceSpec.features.pages.s3` - раздел с настройками объектного хранилища для компонента
        - `settings.instanceSpec.features.pages.s3.mode` - использовать внешнее или встроенное объектное хранилище
        - `settings.instanceSpec.features.pages.s3.bukcetPrefix` - префикс для имени бакетов. Финальные имена будут иметь вид {prefix}-{name}
        - `settings.instanceSpec.features.pages.s3.internal` - в настоящее время не поддерживается. Для будущего использования
        - `settings.instanceSpec.features.pages.s3.external` - раздел с настройками внешнего объектного хранилища
          - `settings.instanceSpec.features.pages.s3.external.endpoint` - пользовательский endpoint для объектного хранилища
          - `settings.instanceSpec.features.pages.s3.external.provider` - провайдер объектного хранилища
          - `settings.instanceSpec.features.pages.s3.external.region` - регион бакетов
          - `settings.instanceSpec.features.pages.s3.external.accessKey` - ключ доступа к бакету
          - `settings.instanceSpec.features.pages.s3.external.secretKey` - секретный ключ для бакета
          - `settings.instanceSpec.features.pages.s3.external.azureAccountName` - имя учетной записи AzureRM
          - `settings.instanceSpec.features.pages.s3.external.azureAccessKey` - ключ доступа к объектному хранилищу AzureRM
          - `settings.instanceSpec.features.pages.s3.external.storageOptions` - раздел с низкоуровневыми настройками шифрования для объектного хранилища компонента
            - `settings.instanceSpec.features.pages.s3.external.storageOptions.serverSideEncryption` - режим шифрования (AES256 или aws:kms)
            - `settings.instanceSpec.features.pages.s3.external.storageOptions.serverSideEncryptionKmsKeyId` - Amazon Resource Name. Необходим только при использовании aws:kms для serverSideEncryption
    - `settings.instanceSpec.features.registry` - раздел с настройками компонента Registry
      - `settings.instanceSpec.features.registry.enabled` - включить ли компонент
      - `settings.instanceSpec.features.registry.ingress` - раздел с настройками входящих сетевых соединений для компонента
        - `settings.instanceSpec.features.registry.ingress.https` - настройки https для входящих соединений компонента
          - `settings.instanceSpec.features.registry.ingress.https.mode` - способ хранения/выдачи сертификата. Может быть custom/certManager/global
          - `settings.instanceSpec.features.registry.ingress.https.certManager.clusterIssuerName` - имя кластерного issuer для SSL-сертификатов
          - `settings.instanceSpec.features.registry.ingress.https.customCertificate.secretName` - имя секрета, где хранится пользовательский сертификат
      - `settings.instanceSpec.features.registry.s3` - раздел с настройками объектного хранилища для компонента
        - `settings.instanceSpec.features.registry.s3.mode` - использовать внешнее или встроенное объектное хранилище для компонента Registry
        - `settings.instanceSpec.features.registry.s3.bucketName` - имя бакета для компонента Registry
        - `settings.instanceSpec.features.registry.s3.external` - раздел с настройками внешнего объектного хранилища
          - `settings.instanceSpec.features.registry.s3.external.provider` - провайдер объектного хранилища
          - `settings.instanceSpec.features.registry.s3.external.endpoint` - пользовательский endpoint для объектного хранилища
          - `settings.instanceSpec.features.registry.s3.external.region` - регион бакетов
          - `settings.instanceSpec.features.registry.s3.external.accessKey` - ключ доступа к бакету
          - `settings.instanceSpec.features.registry.s3.external.secretKey` - секретный ключ для бакета
          - `settings.instanceSpec.features.registry.s3.external.azureAccountName` - имя учетной записи AzureRM
          - `settings.instanceSpec.features.registry.s3.external.azureAccessKey` - ключ доступа к объектному хранилищу AzureRM
        - `settings.instanceSpec.features.registry.s3.internal` - в настоящее время не поддерживается. Для будущего использования
        - `settings.instanceSpec.features.registry.s3.bucketName` - имя бакета, который будет использоваться как объектное хранилище для Registry
      - `settings.instanceSpec.features.registry.maintenance` - раздел с настройками обслуживания Registry
        - `settings.instanceSpec.features.registry.maintenance.readOnly` - перевести ли Registry в режим только для чтения во время обслуживания
        - `settings.instanceSpec.features.registry.maintenance.uploadPurging` - раздел описывает функцию обслуживания, удаляющую артефакты загрузки изображений из хранилища (данные изображений не удаляются)
          - `settings.instanceSpec.features.registry.maintenance.uploadPurging.enabled` - включить ли функцию uploadPurging. Выключено в режиме readOnly
          - `settings.instanceSpec.features.registry.maintenance.uploadPurging.age` - возраст артефактов для удаления, измеряется в часах
          - `settings.instanceSpec.features.registry.maintenance.uploadPurging.interval` - интервал запуска
          - `settings.instanceSpec.features.registry.maintenance.uploadPurging.dryrun` - использовать ли в тестовом/проверочном режиме
