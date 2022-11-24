---
title: "Модуль delivery: пример конфигурации"
---

- [Дисклеймер](#дисклеймер)
- [Концепция](#концепция)
- [Конфигурация](#конфигурация)
  - [WerfSource CRD](#werfsource-crd)
  - [Частичное использование WerfSource CRD](#частичное-использование-werfsource-crd)
    - [Без репозитория Argo CD](#без-репозитория-argo-cd)
    - [Без регистри для Image Updater](#без-регистри-для-image-updater)
- [Публикация артефакта в registry](#публикация-артефакта-в-registry)
- [Деплой](#деплой)
- [Автообновление бандла](#автообновление-бандла)
  - [Правила обновлений образов](#правила-обновлений-образов)
  - [Настройки доступа к registry](#настройки-доступа-к-registry)
    - [Индивидуальная настройка для Application](#индивидуальная-настройка-для-application)
- [Особенности аутентификации CLI](#особенности-аутентификации-cli)
- [Как создать OCI-репозиторий самостоятельно](#как-создать-oci-репозиторий-самостоятельно)
  - [Веб-интерфейс и kubectl](#веб-интерфейс-и-kubectl)
  - [Командная утилита `argocd`](#командная-утилита-argocd)

## Дисклеймер

Мы ожидаем, что читатель знаком с Argo CD, поэтому целью этой статьи является описание особенностей работы в поставке с Deckhouse.

Данные, которые используются в примерах ниже:

- Веб-интерфейс и API Argo CD доступны на адресе `https://argocd.example.com`. Это предполагает, что
  параметр `publicDomainTemplate` выставлен в `%s.example.com`.
- `APP_NAME=myapp` — название приложения.
- `CHART_NAME=mychart` — название Helm-чарта и werf-бандла, в этой схеме они должны
  совпадать. Для явности мы выбрали это название отличным от названия приложения.
- `REGISTRY_HOST=cr.example.com` — хостнейм OCI-регистри.
- `REGISTRY_REPO=cr.example.com/myproject` — репозиторий бандла в OCI-регистри.

## Концепция

Модуль предлагает способ деплоя с помощью связки [werf bundle](https://werf.io/documentation/v1.2/advanced/bundles.html#bundles-publication) и [OCI-based registries](https://helm.sh/docs/topics/registries/).

Преимущество этого подхода заключается в том, что есть единое место доставки артефакта — container
registry. Артефакт содержит в себе как образы контейннеров, так и Helm-чарт. Он использется как для первичного деплоя приложения, так и для автообновлений по pull-модели.

Для continous delivery используется werf. С его опомощью собирается и публикуется артефакт в
регистри (werf bundle). Чтобы использовать OCI-регистри как репозиторий, в параметрах репозитория
Argo CD нужно использовать флаг `enableOCI=true`.

Чтобы автоматически обновлять приложения в кластере после доставки артефакта, используется ArgoCD
Image Updater. Мы используем наш [патч](https://github.com/argoproj-labs/argocd-image-updater/pull/405), чтобы чтобы Image Updater мог работать с werf-бандлами.

Используемые компоненты:

- Argo CD
- Argo CD Image Updater с [патчем для поддержки OCI-репозиториев](https://github.com/argoproj-labs/argocd-image-updater/pull/405)
- werf-argocd-cmp-sidecar, чтобы сохранить аннотации werf во время рендеринга манифестов


![flow](./internal/werf-bundle-and-argocd.png)

В данной схеме изображена схема с паттерном [«Application of
Applications»](https://argo-cd.readthedocs.io/en/stable/operator-manual/cluster-bootstrapping/#app-of-apps-pattern)
подразумевающая два git-репозитория: приложения и инфраструктуры. Инфраструктурный git-репозиторий
необязателен, если допускается создавать ресусры Application вручную. В примерах ниже мы будем
придерживаться ручного управления ресурсами Application для простоты.

## Конфигурация

### WerfSource CRD

Чтобы использовать ArgoCD и Image Updater, достаточно настроить доступ к репозиторию, доступ к
регистри и сам Application. Для этого нужно сконфигурировать

1. секрет для доступа к регистри
2. объект Application с конфигурацией приложения
3. регистри для Image Updater в его configmap, в нем будет ссылка на секрет (1) для регистри
4. секрет репозитрия Argo CD, в неи будет копия параметров доступа из секрета (1)

Данный модуль предлагает более простой способ конфигурации для использования werf bundle и Argo CD.
Упрощение касается двух объектов: репозитория Argo CD и конфигурации регистри для Image Updater.

1. секрет для доступа к регистри в формате `dockerconfigjson`
2. объект Application с конфигурацией приложения
3. объект WerfSource в котором содержится информация о регистри и ссылка на секрет (1) для доступа

Таким образом, для деплоя из OCI-репозитория нужно создать три объекта. Все объекты, предполагающие
область видимости namespace, должны быть в namespace `d8-delivery`.

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: WerfSource
metadata:
  name: example
spec:
  imageRepo: cr.example.io/myproject
  pullSecretName: example-registry
---
apiVersion: v1
kind: Secret
metadata:
  namespace: d8-delivery
  name: example-registry
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: ...
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd-image-updater.argoproj.io/chart-version: ~ 0.0
  name: myapp
  namespace: d8-delivery
spec:
  destination:
    namespace: myapp
    server: https://kubernetes.default.svc
  project: default
  source:
    chart: myapp
    helm: {}
    repoURL: cr.example.com/myproject
    targetRevision: 1.0.0
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - CreateNamespace=true
```

### Частичное использование WerfSource CRD

#### Без репозитория Argo CD

WerfSource отвечает за создание репозитория Argo CD и за внесение регистри в конфигурацию Image
Updater. От создания репозитория можно отказаться в WerfSource, устанловив `spec.argocdRepoEnabled`
в `false`. Это пригодится, если используется тип репозитория, отличный от OCI, например Chart Museum
или git:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: WerfSource
metadata:
  name: example
spec:
  # ...
  argocdRepoEnabled: false
```

#### Без регистри для Image Updater

Регистри в `configmap/argocd-image-updater-config` могут быть настроены только через WerfSource,
потому что deckhouse рендерит этот configmap, используя объекты WerfSource. Если этот способ не
подойдет, то конфигурацию Image Updater можно задать при помощи аннотаций в каждом Application
индивидуально.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd-image-updater.argoproj.io/chart-version: ~ 1.0
    argocd-image-updater.argoproj.io/pull-secret: pullsecret:d8-delivery/example-registry
```

## Публикация артефакта в registry

OCI-чарт хельма требует, чтобы имя чарта в `Chart.yaml` совпадало с последним элементом пути в
OCI-registry. Поэтому название чарта нужно использовать в названии бандла:

```sh
werf bundle publish --repo cr.example.com/myproject/mychart --tag 1.0.0
```

Подробнее о бандлах — в [документации werf: подготовка артефактов
релиза](https://ru.werf.io/documentation/v1.2/advanced/ci_cd/werf_with_argocd/configure_ci_cd.html).

## Деплой

Создайте Application c нужными значениями для чарта.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  # Используйте название ресурса на ваше усмотрение:
  name: myapp
  namespace: d8-delivery
spec:
  destination:
    # Используйте namespace на ваше усмотрение:
    namespace: myapp
    server: https://kubernetes.default.svc
  # Для простоты в этом примере используем проект default
  project: default
  source:
    chart: mychart
    repoURL: cr.example.com/myproject
    targetRevision: 1.0.0
    helm:
      # Переопределение Helm values:
      parameters:
        - name: redis.storage.class
          value: rbd
        - name: http.domain
          value: myapp-api.example.com
  syncPolicy:
    syncOptions:
      - CreateNamespace=true
```

## Автообновление бандла

ArgoCD Image Updater используется для автоматического обновления Application из опубликованного
werf-бандла в pull-модели. Image Updater сканирует OCI-репозиторий с заданным интервалом и обновляет
`targetRevision` в Application, посредством чего обновляется все приложение из обновленного
арттефакта. Мы используем пропатченный форк Image Updater, который умеет работать с OCI-регистри, и,
соответственно, с werf-бандлами.

### Правила обновлений образов

В Application нужно добавить аннотацию с правилами обновления образа
([документация](https://argocd-image-updater.readthedocs.io/en/stable/basics/update-strategies/)).
Пример правила, обновляющие патч-версии приложения `1.0.*`:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd-image-updater.argoproj.io/chart-version: ~ 1.0
```

### Настройки доступа к registry

У сервисаккаунта `argocd-image-updater` есть права работу ресурсами только в неймспейсе
`d8-delivery`, поэтому именно в нем необходимо создать секрет с параметрами доступа к регистри, на
который ссылается поле `credetials`.

#### Индивидуальная настройка для Application

Сослаться на параметры доступа можно индивидуально в каждом Application с помощью аннотации:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd-image-updater.argoproj.io/chart-version: ~ 1.0
    argocd-image-updater.argoproj.io/pull-secret: pullsecret:d8-delivery/example-registry
```

## Особенности аутентификации CLI

Доступные варианты

1. Использовать username и password из конфигурации ArgoCD или для пользователя `admin` (по
   умолчанию выключен).
2. Через kubectl, если настроен внешний доступ к Kubernetes API (TODO ссыль на
   user-authn/publishAPI)
   ```sh
   argocd login argocd.example.com --core
   ```

Авторизация через Dex не работает для CLI, хотя работает в веб-интерфейсе.

```sh
argocd login argocd.example.com --sso # не работает
```

## Как создать OCI-репозиторий самостоятельно


### Веб-интерфейс и kubectl

Регистри играет роль репозитория бандлов. Чтобы это работало, нужно в репозитории включить режим
OCI. Однако веб-интерфейс не позволяет установить флаг `enableOCI`, поэтому его нужно добавить
вручную в рекрет с репозиторием:

```sh
kubectl -n d8-delivery edit secret repo-....
```

```yaml
apiVersion: v1
kind: Secret
stringData:           # <----- добавить
  enableOCI: "true"   # <----- и сохранить
data:
  # (...)
metadata:
  # (...)
  name: repo-....
  namespace: d8-delivery
type: Opaque
```
### Командная утилита `argocd`

Утилита `argocd` [не позволяет указывать namespace](https://github.com/argoproj/argo-cd/issues/9123)
во время вызова и рассчитывает на namespace `argocd`. Модуль с ArgoCD в Deckhouse находится в
namespace `d8-delivery`. Поэтому namespace `d8-delivery` нужно назначить по умолчанию в kubectl,
чтобы подключаться к ArgoCD:

```sh
# временно переключите namespace по умолчанию
$ kubectl config set-context --current --namespace=d8-delivery

# создайте конфигурацию репозитория для бандлов
$ argocd repo add cr.example.com/myproject \
  --enable-oci \
  --type helm \
  --name REPO_NAME \
  --username USERNAME \
  --password PASSWORD
```
