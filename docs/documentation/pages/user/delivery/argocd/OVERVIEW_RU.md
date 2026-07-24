---
title: "Доставка приложений с помощью Argo CD"
permalink: ru/user/delivery/argocd/
description: "Доставка приложений с помощью Argo CD в Deckhouse Kubernetes Platform."
lang: ru
search: argocd, доставка приложений
relatedLinks:
  - title: "Официальный сайт проекта Argo CD"
    url: "https://argo-cd.readthedocs.io"
  - title: "Официальный сайт проекта Argo CD Operator"
    url: "https://argocd-operator.readthedocs.io"
---

В этом разделе описано, как организовать доставку приложений с помощью Argo CD в Deckhouse Kubernetes Platform (DKP).

Argo CD позволяет описывать приложения декларативно и синхронизировать их состояние с содержимым Git-репозитория. Пользователь задаёт источник манифестов, целевой кластер, неймспейс и параметры синхронизации, после чего Argo CD развёртывает приложение и поддерживает его в целевом состоянии.

В DKP экземпляры Argo CD разворачиваются с помощью модуля [operator-argo](/modules/operator-argo/). Пользовательская работа с Argo CD обычно включает:

- создание или использование существующего проекта [`AppProject`](/modules/operator-argo/cr.html#appproject);
- подготовку целевого неймспейса для приложения;
- создание ресурса [`Application`](/modules/operator-argo/cr.html#application), описывающего источник приложения и правила синхронизации;
- создание приложения через веб-интерфейс Argo CD;
- создание приложения с помощью CLI-утилиты `argocd`.

## Предварительные условия

Перед началом работы должны быть выполнены следующие условия:

- администратор кластера включил модуль [operator-argo](/modules/operator-argo/);
- администратор развернул хотя бы один экземпляр Argo CD;
- у пользователя есть доступ к нужному экземпляру Argo CD и целевым неймспейсам.

Если экземпляр Argo CD ещё не развёрнут, обратитесь к администратору или воспользуйтесь инструкцией из раздела [«Запуск Argo CD»](/products/kubernetes-platform/documentation/v1/admin/configuration/delivery/argocd/setup.html).

## Проекты `AppProject`

[`AppProject`](/modules/operator-argo/cr.html#appproject) — это ресурс Argo CD, который задаёт логические границы проекта. С его помощью можно определить:

- какие Git-репозитории разрешено использовать как источник приложений;
- в какие кластеры и неймспейсы разрешено развёртывать приложения;
- какие кластерные и namespaced-ресурсы разрешено использовать;
- какие роли и политики доступа действуют внутри проекта.

Каждый ресурс [`Application`](/modules/operator-argo/cr.html#application) должен ссылаться на проект через поле `spec.project`.

По умолчанию в Argo CD существует проект `default`. Его можно использовать для первых экспериментов и простых сценариев. Для production-окружений рекомендуется создать отдельные проекты `AppProject`, чтобы ограничить доступ к репозиториям, кластерам и неймспейсам.

Манифест `AppProject` `default` приведён ниже:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: default
  namespace: argocd
spec:
  clusterResourceWhitelist:
  - group: '*'
    kind: '*'
  destinations:
  - namespace: '*'
    server: '*'
  sourceRepos:
  - '*'
```

## Подготовка неймспейса

Перед развёртыванием приложения создайте целевой неймспейс и добавьте лейбл `argocd.argoproj.io/managed-by`, указывающий, какой экземпляр Argo CD управляет ресурсами в этом неймспейсе.

Пример:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: demo
  labels:
    argocd.argoproj.io/managed-by: argocd
```

В этом примере неймспейс `demo` будет управляться экземпляром Argo CD, развёрнутым в неймспейсе `argocd`.

## Развёртывание приложения

Приложение в Argo CD можно создать несколькими способами:

- декларативно — с помощью ресурса [`Application`](/modules/operator-argo/cr.html#application);
- интерактивно — через веб-интерфейс Argo CD;
- с помощью CLI-утилиты `argocd`.

### Создание приложения с помощью ресурса `Application`

Для описания приложения используется ресурс [`Application`](/modules/operator-argo/cr.html#application). В нём указываются:

- проект Argo CD (`spec.project`);
- источник манифестов или чарта (`spec.source`);
- целевой кластер и неймспейс (`spec.destination.server` и `spec.destination.namespace`);
- политика синхронизации (`spec.syncPolicy`).

Пример ресурса `Application`:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: demo
  namespace: argocd
spec:
  destination:
    namespace: demo
    server: https://kubernetes.default.svc
  project: default
  source:
    path: helm-guestbook
    repoURL: https://github.com/argoproj/argocd-example-apps
    targetRevision: HEAD
  syncPolicy:
    # Включить автоматическую синхронизацию.
    automated:
      # Удалять устаревшие ресурсы.
      prune: true
      # Включить самовосстановление в случае сторонних изменений.
      selfHeal: true
```

После создания ресурса `Application` Argo CD начнёт отслеживать состояние приложения и синхронизировать его с содержимым репозитория. В результате в неймспейсе `demo` должны появиться ресурсы, относящиеся к приложению `demo`:

```console
d8 k -n demo get deployment,svc,pod
NAME                                  READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/demo-helm-guestbook   1/1     1            1           15s

NAME                          TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)   AGE
service/demo-helm-guestbook   ClusterIP   10.222.177.84   <none>        80/TCP    15s

NAME                                       READY   STATUS    RESTARTS   AGE
pod/demo-helm-guestbook-66d5d69ccd-xfkb7   1/1     Running   0          15s
```

### Создание приложения через веб-интерфейс Argo CD

Приложение можно создать через веб-интерфейс Argo CD. Для этого:

1. Откройте веб-интерфейс нужного экземпляра Argo CD.
1. Перейдите в раздел **Applications**.
1. Нажмите **New App**.
1. Укажите имя приложения, проект, репозиторий, ревизию, путь к манифестам или чарту, а также целевой кластер и неймспейс.
1. При необходимости настройте автоматическую синхронизацию и дополнительные параметры.
1. Нажмите **Create**.

Поля формы в веб-интерфейсе соответствуют основным полям ресурса `Application`: проекту, источнику приложения, целевому кластеру, неймспейсу и политике синхронизации.

### Создание приложения с помощью CLI-утилиты `argocd`

CLI-утилита `argocd` позволяет создавать и сопровождать приложения из командной строки. Бинарный файл `argocd` можно загрузить из раздела **Documentation** веб-интерфейса Argo CD.

Перед созданием приложения аутентифицируйтесь с помощью команды:

```bash
argocd login <argocd-domain>:443
```

{% alert level="info" %}
При использовании аутентификации через SSO добавьте к команде логина флаг `--sso`.
{% endalert %}

Пример создания приложения:

```bash
argocd app create guestbook \
  --repo https://github.com/argoproj/argocd-example-apps.git \
  --path guestbook \
  --dest-namespace demo \
  --dest-server https://kubernetes.default.svc \
  --directory-recurse \
  --sync-policy automated \
  --self-heal \
  --auto-prune
```

Как и в случае с созданием ресурса `Application`, в команде указываются источник приложения, целевой кластер, неймспейс и политика синхронизации.

Чтобы просмотреть статус развёрнутого приложения, используйте команду `argocd apps get <название приложения>`, например:

```console
argocd app get guestbook
Name:               argocd/guestbook
Project:            default
Server:             https://kubernetes.default.svc
Namespace:          demo
URL:                https://argocd.192.168.0.235.sslip.io/applications/guestbook
Source:
- Repo:             https://github.com/argoproj/argocd-example-apps.git
  Target:
  Path:             guestbook
SyncWindow:         Sync Allowed
Sync Policy:        Automated (Prune)
Sync Status:        Synced to  (8088f4c)
Health Status:      Healthy

GROUP  KIND        NAMESPACE  NAME          STATUS  HEALTH   HOOK  MESSAGE
apps   Deployment  demo       guestbook-ui  Synced  Healthy        deployment.apps/guestbook-ui unchanged
       Service     demo       guestbook-ui  Synced  Healthy
```
