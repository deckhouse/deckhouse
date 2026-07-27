---
title: "Запуск Argo CD"
permalink: ru/admin/configuration/delivery/argocd/setup/
description: "Запуск Argo CD в Deckhouse Kubernetes Platform."
lang: ru
relatedLinks:
  - title: "Официальный сайт Argo CD"
    url: "https://argo-cd.readthedocs.io"
  - title: "Официальный сайт Argo CD Operator"
    url: "https://argocd-operator.readthedocs.io"
---

В этом разделе описаны шаги, необходимые для запуска экземпляра Argo CD в кластере DKP:

- [Подготовка к запуску Argo CD](#подготовка-к-запуску-argo-cd)
- Развёртывание [одного](#развёртывание-экземпляра-argo-cd) или [нескольких](#развёртывание-нескольких-экземпляров-argo-cd) экземпляров Argo CD
- [Дополнительные настройки](#расширенные-настройки) Argo CD

## Подготовка к запуску Argo CD

Прежде чем создавать экземпляры Argo CD, выполните следующие шаги:

1. [Включите модуль `operator-argo`](/modules/operator-argo/configuration.html#enable).

1. Дождитесь, пока модуль перейдёт в состояние `Ready`.

   Проверить состояние модуля можно в веб-интерфейсе DKP или с помощью команды:

   ```bash
   d8 k get module operator-argo -w
   ```

Подробная информация о настройках модуля приведена в [документации модуля `operator-argo`](/modules/operator-argo/).

После включения модуля `operator-argo` в кластере DKP станут доступны кастомные ресурсы Argo CD.

Чтобы запустить экземпляр Argo CD, необходимо создать объект [ArgoCD](/modules/operator-argo/cr.html#argocd).
Подробнее запуск [одного](#развёртывание-экземпляра-argo-cd) или [нескольких](#развёртывание-нескольких-экземпляров-argo-cd) экземпляров Argo CD рассмотрен далее.
Работа с кастомными ресурсами, относящимися к области экземпляра Argo CD, описана в [разделе «Использование»](/products/kubernetes-platform/documentation/v1/user/delivery/argocd/).

{% offtopic title="Список кастомных ресурсов Argo CD, создаваемых модулем `operator-argo`..." %}

| CRD | Область использования | Назначение                                                                                                                                                                                                                                                              |
|---|---|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [`argocds.argoproj.io`](/modules/operator-argo/cr.html#argocd) | **Модуль `operator-argo`** | Кастомный ресурс, по которому модуль `operator-argo` настраивает и сопровождает экземпляр Argo CD в целевой среде. Через объект ArgoCD задаются параметры установки, состав компонентов, интеграции, политики доступа и другие настройки экземпляра.                  |
| [`appprojects.argoproj.io`](/modules/operator-argo/cr.html#appproject) | **Экземпляр Argo&nbsp;CD** | Используется для логической сегментации приложений и задания политик: допустимых Git-репозиториев, целевых кластеров и неймспейсов, а также правил доступа и ограничений на использование ресурсов.                                                                     |
| [`applications.argoproj.io`](/modules/operator-argo/cr.html#application) | **Экземпляр Argo&nbsp;CD** | Основная прикладная CRD Argo CD для описания приложения, которое должно синхронизироваться из декларативного источника (Git, Helm, Kustomize и другие) в Kubernetes. Определяет источник, целевое окружение и параметры синхронизации.                                 |
| [`applicationsets.argoproj.io`](/modules/operator-argo/cr.html#applicationset) | **Экземпляр Argo&nbsp;CD** | Используется для автоматизированного создания набора объектов Application по шаблону. Подходит для сценариев массового управления приложениями: в нескольких кластерах, окружениях, директориях, командах или ветках репозитория.                                     |
| [`argocdexports.argoproj.io`](/modules/operator-argo/cr.html#argocdexport) | **Экземпляр Argo&nbsp;CD** | Используется для декларативного экспорта данных, связанных с экземпляром Argo CD, во внешние системы или смежные компоненты DKP. Обычно применяется в интеграционных сценариях, где требуется формализованно публиковать сведения о конфигурации, статусе или доступах. |
| [`namespacemanagements.argoproj.io`](/modules/operator-argo/cr.html#namespacemanagement) | **Экземпляр Argo&nbsp;CD** | Используется для управления жизненным циклом неймспейсов в рамках GitOps-процессов. Может автоматизировать создание, настройку и сопровождение неймспейсов, в которые затем выполняется развёртывание приложений.                                                       |
| [`notificationsconfigurations.argoproj.io`](/modules/operator-argo/cr.html#notificationsconfiguration) | **Экземпляр Argo&nbsp;CD** | Используется для настройки механизма уведомлений Argo&nbsp;CD. Позволяет декларативно описывать каналы доставки и правила отправки событий, связанных с синхронизацией, ошибками развёртывания, изменением статуса приложений и другими операционными событиями.        |
| [`imageupdaters.argocd-image-updater.argoproj.io`](/modules/operator-argo/cr.html#imageupdater) | **Экземпляр Argo&nbsp;CD** | Используется для автоматического отслеживания новых версий контейнерных образов и обновления параметров приложений в соответствии с заданной стратегией версионирования и публикации.                                                                                   |
{% endofftopic %}

## Развёртывание экземпляра Argo CD

Параметры, доступные для конфигурации экземпляра Argo CD, приведены [в документации модуля `operator-argo`](/modules/operator-argo/cr.html#argocd).

Чтобы развернуть экземпляр Argo CD в неймспейсе `argocd` и опубликовать веб-интерфейс Argo CD через Ingress, используйте следующий пример (укажите собственные значения для `<ARGOCD_DOMAIN>` и `<TLS_SECRET_NAME>`):

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: argocd
---
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  server:
    host: <ARGOCD_DOMAIN>
    ingress:
      enabled: true
      tls:
        - hosts:
            - <ARGOCD_DOMAIN>
          secretName: <TLS_SECRET_NAME>
    insecure: true
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: argocd-ingress
  namespace: argocd
spec:
  dnsNames:
    - <ARGOCD_DOMAIN>
  issuerRef:
    kind: ClusterIssuer
    name: letsencrypt
  secretName: <TLS_SECRET_NAME>
```

{% alert level="warning" %}
Параметр [`spec.server.insecure: true`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-server-insecure) в примере выше отключает внутренний TLS у API-сервера Argo CD. Это позволяет избежать циклических перенаправлений при публикации через Ingress.
{% endalert %}

После создания объекта ArgoCD из примера выше в неймспейсе `argocd` будут запущены компоненты Argo CD:

```console
d8 k -n argocd get pods
NAME                                  READY   STATUS    RESTARTS   AGE
argocd-application-controller-0       1/1     Running   0          35m
argocd-dex-server-759fff8444-zglp4    1/1     Running   0          2d23h
argocd-redis-568f5b889c-jg5dr         1/1     Running   0          4d
argocd-repo-server-78d9d6bcc6-9rwcm   1/1     Running   0          3d22h
argocd-server-76597597f9-kfqdl        1/1     Running   0          35m
podinfo-ccdb96645-zv5tm               1/1     Running   0          2d20h
```

Когда все компоненты перейдут в статус `Running`, веб-интерфейс экземпляра Argo CD станет доступен по адресу, указанному в параметре [`spec.server.ingress.tls.hosts`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-server-ingress-tls-hosts) (в примере — `https://<ARGOCD_DOMAIN>`).
Настройка аутентификации и получения учётных данных описана в разделе [«Настройка аутентификации и авторизации»](../authentication/).

Доставка пользовательских приложений с помощью Argo CD описана в [разделе «Использование»](../../../../../user/delivery/argocd/).

{% alert level="info" %}
Для работы с Argo CD можно использовать не только веб-интерфейс и кастомные ресурсы, но и CLI-утилиту `argocd`. Бинарный файл можно загрузить из раздела «Documentation» веб-интерфейса Argo CD. Инструкцию по работе с CLI-утилитой можно получить, выполнив `argocd --help`.
{% endalert %}

## Развёртывание нескольких экземпляров Argo CD

Если в кластере требуется несколько экземпляров Argo CD, создайте для каждого из них отдельный неймспейс (или проект DKP) и отдельный объект ArgoCD.

Например, создайте:

- отдельный экземпляр для production-окружения;
- отдельный экземпляр для тестовых окружений;
- отдельный экземпляр для конкретной команды или проекта.

При таком подходе каждый экземпляр Argo CD управляется независимо и имеет собственную конфигурацию.

{% alert level="warning" %}
В одном неймспейсе поддерживается создание не более одного объекта ArgoCD.
{% endalert %}

## Расширенные настройки

### Включение режима высокой доступности

Чтобы включить режим высокой доступности, установите параметр [`spec.ha.enabled: true`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-ha-enabled) в объекте ArgoCD.

Пример:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  ha:
    enabled: true
    resources:
      requests:
        cpu: "500m"
        memory: "512Mi"
      limits:
        cpu: "1"
        memory: "1Gi"
```

{% alert level="warning" %}
Режим высокой доступности обеспечивает высокую доступность только для хранилища состояния `Redis` с использованием `HAProxy`. Он не делает все компоненты Argo CD высокодоступными автоматически.
{% endalert %}

{% alert level="info" %}
Для запуска Argo CD в режиме высокой доступности требуется не менее трёх узлов кластера из-за правил `pod anti-affinity`. Кластеры, работающие только с IPv6, не поддерживаются.

При включённом режиме высокой доступности изменения в [`.spec.redis.resources`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-redis-resources) не применяются. Ограничения и запросы ресурсов для `Redis` настраивайте через параметр [`.spec.ha.resources`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-ha-resources).
{% endalert %}

### Предоставление доступа к кластерным ресурсам

По умолчанию экземпляр Argo CD получает привилегии только на неймспейс, в котором он запущен, и неймспейсы, помеченные лейблом `argocd.argoproj.io/managed-by`. Значение лейбла должно совпадать с названием неймспейса, в котором запущен экземпляр Argo CD.

Чтобы разрешить создание cluster-wide-ресурсов, укажите в параметре [`clusterConfigNamespaces`](/modules/operator-argo/configuration.html#parameters-clusterconfignamespaces) настроек модуля `operator-argo` имя неймспейса, в котором запущен целевой экземпляр Argo CD.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: operator-argo
spec:
  enabled: true
  settings:
    clusterConfigNamespaces: argocd
  version: 1
```

Если экземпляров Argo CD несколько, перечислите их в параметре `clusterConfigNamespaces` через запятую.

После внесения изменений в `ModuleConfig` автоматически будут созданы следующие кластерные роли (ClusterRole) и кластерные биндинги (ClusterRoleBinding):

```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    argocds.argoproj.io/name: argocd
    argocds.argoproj.io/namespace: argocd
  labels:
    app.kubernetes.io/managed-by: argocd
    app.kubernetes.io/name: argocd
    app.kubernetes.io/part-of: argocd
  name: argocd-argocd-argocd-application-controller
rules:
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - '*'
- apiGroups:
  - ""
  resources:
  - serviceaccounts
  verbs:
  - impersonate
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    argocds.argoproj.io/name: argocd
    argocds.argoproj.io/namespace: argocd
  labels:
    app.kubernetes.io/managed-by: argocd
    app.kubernetes.io/name: argocd-application-controller
    app.kubernetes.io/part-of: argocd
  name: argocd-argocd-argocd-application-controller

roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: argocd-argocd-argocd-application-controller
subjects:
- kind: ServiceAccount
  name: argocd-argocd-application-controller
  namespace: argocd
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    argocds.argoproj.io/name: argocd
    argocds.argoproj.io/namespace: argocd
  labels:
    app.kubernetes.io/managed-by: argocd
    app.kubernetes.io/name: argocd
    app.kubernetes.io/part-of: argocd
  name: argocd-argocd-argocd-server
rules:
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - get
  - delete
  - patch
- apiGroups:
  - argoproj.io
  resources:
  - applications
  - applicationsets
  verbs:
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - list
- apiGroups:
  - batch
  resources:
  - jobs
  - cronjobs
  - cronjobs/finalizers
  verbs:
  - create
  - update
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    argocds.argoproj.io/name: argocd
    argocds.argoproj.io/namespace: argocd
  labels:
    app.kubernetes.io/managed-by: argocd
    app.kubernetes.io/name: argocd-server
    app.kubernetes.io/part-of: argocd
  name: argocd-argocd-argocd-server
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: argocd-argocd-argocd-server
subjects:
- kind: ServiceAccount
  name: argocd-argocd-server
  namespace: argocd
```

{% alert level="info" %}
Если полномочия, описанные в кластерных ролях по умолчанию, избыточны, в настройках объекта ArgoCD укажите [`spec.defaultClusterScopedRoleDisabled: true`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-defaultclusterscopedroledisabled). В этом случае кластерные роли не будут созданы автоматически, и вы сможете самостоятельно описать необходимый уровень привилегий для ServiceAccount, используемого экземпляром Argo CD.
{% endalert %}

{% alert level="info" %}
При необходимости уровень доступа к кластерным ресурсам можно переопределить на уровне объекта [AppProject](/modules/operator-argo/cr.html#appproject), используя параметры [`spec.clusterResourceBlacklist`](/modules/operator-argo/cr.html#appproject-v1alpha1-spec-clusterresourceblacklist) и [`spec.clusterResourceWhitelist`](/modules/operator-argo/cr.html#appproject-v1alpha1-spec-clusterresourcewhitelist).
{% endalert %}

### Использование собственного домена кластера

Если в кластере используется домен, отличный от `cluster.local`, укажите его в параметре [`spec.clusterDomain`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-clusterdomain) объекта ArgoCD.

Пример:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  clusterDomain: prod.local
```
