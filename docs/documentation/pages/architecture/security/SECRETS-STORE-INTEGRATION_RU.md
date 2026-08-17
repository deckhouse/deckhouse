---
title: Модуль secrets-store-integration
permalink: ru/architecture/security/secrets-store-integration.html
lang: ru
search: vault, secrets
description: Архитектура модуля secrets-store-integration в Deckhouse Kubernetes Platform.
---

Модуль [`secrets-store-integration`](/modules/secrets-store-integration/) реализует доставку секретов для приложений в Deckhouse Kubernetes Platform (DKP) из внешнего хранилища, совместимого с API [HashiCorp Vault](https://github.com/hashicorp/vault).

Модуль предоставляет следующие возможности:

- доставка секретов в поды в виде примонтированных файлов или переменных окружения без хранения в etcd;
- автоматическое подключение к внутреннему экземпляру [Deckhouse Stronghold](/products/stronghold/documentation/) без ручной настройки (режим `DiscoverLocalStronghold`);
- работа с любым Vault-совместимым хранилищем секретов в режиме `Manual`;
- автоматическое обновление примонтированных секретов каждые две минуты при изменении значения в хранилище;
- подмена команды запуска (entrypoint) для приложений, которые нельзя модифицировать для прямого чтения секретов из хранилища;
- доставка бинарных секретов в формате Base64 (например, JKS-хранилища, keytab-файлы для Kerberos) с автоматическим раскодированием.

Режим работы (`Manual` или `DiscoverLocalStronghold`) задаётся параметром модуля [`settings.connectionConfiguration`](/modules/secrets-store-integration/configuration.html#parameters-connectionconfiguration) кастомного ресурса [ModuleConfig](../../reference/api/cr.html#moduleconfig).

Модуль работает со следующими кастомными ресурсами:

- SecretProviderClass — описывает, какие секреты и из какого внешнего хранилища нужно доставлять в под. В спецификации этого ресурса также определяются параметры подключения к источнику секретов и сопоставление путей в контейнере;
- SecretProviderClassPodStatus — содержит статус процесса монтирования секретов в под и диагностическую информацию;
- [SecretsStoreImport](/modules/secrets-store-integration/alpha/cr.html#secretsstoreimport) — хранит сопоставление секретов между Vault-совместимым хранилищем и файлами в контейнерах.

Подробнее с описанием модуля можно ознакомиться в [соответствующем разделе документации](/modules/secrets-store-integration/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

- На схеме контейнеры разных подов показаны как взаимодействующие напрямую. Фактически обмен выполняется через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса приводится над стрелкой.
- Поды могут быть запущены в нескольких репликах, однако на схеме каждый под показан в единственном экземпляре.
{% endalert %}

Архитектура модуля [`secrets-store-integration`](/modules/secrets-store-integration/) на уровне 2 модели C4 и его взаимодействия с другими компонентами DKP изображены на следующей диаграмме:

![Архитектура модуля secrets-store-integration](../../images/architecture/security/c4-l2-secrets-store-integration.ru.svg)

## Компоненты модуля

Модуль `secrets-store-integration` состоит из следующих компонентов:

1. **Csi-secrets-store** (DaemonSet) — компонент работает на всех узлах кластера и реализует стандарт [Container Storage Interface (CSI)](https://github.com/container-storage-interface/spec/blob/master/spec.md) для обеспечения доставки секретов в под в виде примонтированных файлов.

   Csi-secrets-store выполняет следующие действия:

   - регистрирует эфемерный CSI-драйвер `secrets-store.csi.deckhouse.io` в [kubelet](../kubernetes-and-scheduling/kubelet.html);
   - взаимодействует с vault-csi-provider для получения данных из хранилища секретов;
   - монтирует секреты в виде файлов в под;
   - выполняет ротацию секретов;
   - отслеживает ресурсы Pod, CSIDriver и кастомный ресурс SecretProviderClass;
   - работает с кастомным ресурсом SecretProviderClassPodStatus.

   Компонент содержит следующие контейнеры:

   - **injector-puller** — init-контейнер, который выполняет однократный запуск файл-инжектора (исполняемого файла для безопасного внедрения секретов из внешнего хранилища в окружение приложения внутри пода) из служебного образа `secrets-store-integration/env-injector`. Однократный запуск необходим для предварительной загрузки этого образа на каждый узел кластера. Образ используется компонентом webhook для доставки секретов в пользовательское приложение в виде переменных окружения.
   - **csi-node-driver-registrar** — сайдкар-контейнер, регистрирующий CSI Node Plugin в kubelet. Вызывает RPC `GetPluginInfo` и `NodeGetInfo` в контейнере secrets-store, чтобы получить информацию о плагине и узле;
   - **secrets-store** — основной контейнер;
   - **csi-livenessprobe** — сайдкар-контейнер, который отслеживает состояние Unix-сокета CSI-драйвера и предоставляет HTTP-эндпоинт /healthz, за которым следит kubelet. При неуспешном выполнении проверки `livenessProbe` kubelet перезапускает под csi-secrets-store;
   - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищённого доступа к метрикам контейнера secrets-store.

1. **Vault-csi-provider** (DaemonSet) — компонент работает на всех узлах кластера и состоит из одного контейнера **vault-csi-provider**. Vault-csi-provider авторизуется в хранилище секретов, получает из него данные и передаёт их csi-secrets-store.

1. **Ssi-controller** (Deployment) — контроллер состоит из одного контейнера **ssi-controller**. Контроллер отслеживает кастомные ресурсы SecretsStoreImport и создаёт на их основе кастомные ресурсы SecretProviderClass.

1. **Webhook** (Deployment) — компонент реализует mutating-вебхуки для ресурсов Pod через механизм [Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/).

   Компонент изменяет манифест пода, если задана аннотация пода `secrets-store.deckhouse.io/role`. При этом компонент выполняет следующие модификации манифеста пода:

   - добавляет init-контейнер, который копирует из служебного образа `secrets-store-integration/env-injector` статически собранный файл-инжектор во временную директорию, общую для всех контейнеров пода;
   - если в манифесте пода задана аннотация `secrets-store.deckhouse.io/env-from-path` или контейнер использует секреты из хранилища, то для каждого такого контейнера, включая init-контейнеры, компонент заменяет оригинальную команду запуска на запуск файл-инжектора.

     В качестве аргумента файл-инжектор получает оригинальную команду запуска. Если в манифесте пода у контейнера отсутствует команда запуска, компонент получает команду запуска из образа в хранилище образов контейнеров.

   Компонент содержит следующие контейнеры:

   - **vault-secrets-webhook** — основной контейнер;
   - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищённого доступа к метрикам контейнера vault-secrets-webhook.

Следующие компоненты не входят в состав модуля, но модуль влияет на них:

- **User-app** — компонент представляет типовое приложение пользователя, в которое требуется доставить секреты в переменные окружения.

   Компонент содержит следующие контейнеры:

  - **copy-env-injector** — init-контейнер, добавленный компонентом webhook, выполняет копирование исполняемого файл-инжектора из служебного образа `secrets-store-integration/env-injector`;
  - **&lt;CONTAINER_NAME&gt;** — один или несколько контейнеров (в том числе init-контейнеры) оригинального приложения пользователя, команда запуска которых изменена компонентом webhook на запуск файл-инжектора.

   Файл-инжектор авторизуется в хранилище секретов, получает данные, передаёт секреты приложению через переменные окружения и запускает исходную команду контейнера. Если указана аннотация `secrets-store.deckhouse.io/restart-on-secret-change` со значением `watch-for-lease` или `watch-for-data`, файл-инжектор выполняет ротацию секретов при их изменении в хранилище.

## Взаимодействия модуля

Модуль `secrets-store-integration` взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

   - авторизация запросов;
   - получение CSIDriver, Secret, ConfigMap и ServiceAccount;
   - работа с кастомными ресурсами SecretsStoreImport, SecretProviderClass и SecretProviderClassPodStatus.

1. **Хранилище образов** — получение команды запуска из образа пользовательского контейнера.

1. **Хранилище секретов** — авторизация запросов и получение секретов.

С модулем взаимодействуют следующие внешние компоненты:

1. **Kube-apiserver** — вызов mutating-вебхука при создании ресурса Pod.

1. **Kubelet**:

   - регистрирует Node Plugin;
   - проверяет `livenessProbe` CSI-драйвера;
   - вызывает RPC `NodePublishVolume` и `NodeUnpublishVolume` в Node Plugin.

1. **Prometheus-main** — собирает метрики компонентов csi-secrets-store и webhook.
