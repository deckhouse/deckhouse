---
title: "Модуль delivery: примеры конфигурации"
---

## Прежде чем начать

Раздел описывает особенности работы Argo CD в поставке с Deckhouse и предполагает наличие базовых знаний или предварительного знакомства с Argo CD.

Данные, которые используются в примерах ниже:
- для доступа к веб-интерфейсу и API Argo CD выделен домен `argocd` в соответствии с шаблоном имен, определенным в параметре [publicDomainTemplate](../../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate). В примерах ниже используется адрес `argocd.example.com`;
- `myapp` — название приложения;
- `mychart` — название Helm-чарта и werf-бандла. В приведенной схеме они должны совпадать. Для большей ясности название Helm-чарта и werf-бандла выбрано отличным от названия приложения;
- `cr.example.com` — хостнейм OCI-регистри;
- `cr.example.com/myproject` — репозиторий бандла в OCI-регистри.

## Концепция

Модуль предлагает способ развертывания приложений с помощью связки
[werf bundle](https://ru.werf.io/documentation/v1.2/advanced/bundles.html#выкат-бандлов)
и [OCI-based registries](https://helm.sh/docs/topics/registries/).

Преимущество этого подхода заключается в том, что есть единое место доставки артефакта — container
registry. Артефакт содержит в себе как образы контейнеров, так и Helm-чарт. Он используется как для
первичного деплоя приложения, так и для автообновлений по pull-модели.

Используемые компоненты:

- Argo CD;
- Argo CD Image Updater с [патчем для поддержки OCI-репозиториев](https://github.com/argoproj-labs/argocd-image-updater/pull/405);
- werf-argocd-cmp-sidecar, чтобы сохранить аннотации werf во время рендеринга манифестов.

Чтобы использовать OCI-registry как репозиторий, в параметрах репозитория Argo CD нужно использовать
флаг `enableOCI=true`. Модуль `delivery` его устанавливает автоматически.

Чтобы автоматически обновлять приложения в кластере после доставки артефакта, используется Argo CD
Image Updater. В Argo CD Image Updater внесены [изменения](https://github.com/argoproj-labs/argocd-image-updater/pull/405), позволяющие ему работать с werf-бандлами.

В примерах используется схема с шаблоном [«Application of
Applications»](https://argo-cd.readthedocs.io/en/stable/operator-manual/cluster-bootstrapping/#app-of-apps-pattern),
подразумевающая два git-репозитория — отдельный репозиторий для приложения и отдельный репозиторий для инфраструктуры:
![flow](../../images/502-delivery/werf-bundle-and-argocd.svg)

Использование шаблона Application of Applications и отдельного репозитория для инфраструктуры не обязательно, если допускается создавать
ресурсы Application вручную. Для простоты в примерах ниже мы будем придерживаться ручного управления ресурсами Application.

## Конфигурация с WerfSource CRD

Чтобы использовать Argo CD и Argo CD Image Updater, достаточно настроить объект Application и доступ к registry.
Доступ к registry нужен в двух местах — в репозитории Argo CD и в конфигурации Argo CD
Image Updater. Для этого нужно сконфигурировать:

1. Secret для доступа к registry.
2. Объект Application с конфигурацией приложения.
3. Registry для Image Updater в его configMap, в нем будет ссылка на Secret (п. 1) для registry.
4. Secret репозитория Argo CD, в нем будет копия параметров доступа из Secret'а (п. 1).

Модуль `delivery` упрощает конфигурацию для использования werf bundle и Argo CD. Упрощение касается двух
объектов: репозитория Argo CD и конфигурации registry для Image Updater. Эта конфигурация задается
одним ресурсом — *WerfBundle*. Поэтому в рамках модуля нужно определить конфигурацию в трех местах:

1. Secret для доступа к registry в формате `dockerconfigjson`.
2. Объект Application с конфигурацией приложения.
3. Объект WerfSource, содержащий информацию о registry и ссылку на Secret (п. 1) для доступа

Таким образом, для деплоя из OCI-репозитория нужно создать три объекта. Все объекты, предполагающие
область видимости namespace, должны быть созданы в namespace `d8-delivery`.

Пример:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: WerfSource
metadata:
  name: example
spec:
  imageRepo: cr.example.io/myproject  # Репозиторий бандлов и образов.
  pullSecretName: example-registry    # Secret с доступом.
---
apiVersion: v1
kind: Secret
metadata:
  namespace: d8-delivery              # Namespace модуля.
  name: example-registry
type: kubernetes.io/dockerconfigjson  # Поддерживается только этот тип Secret'ов.
data:
  .dockerconfigjson: ...
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd-image-updater.argoproj.io/chart-version: ~ 0.0
  name: myapp
  namespace: d8-delivery  # Namespace модуля.
spec:
  destination:
    namespace: myapp
    server: https://kubernetes.default.svc
  project: default
  source:
    chart: mychart                    # Бандл — cr.example.com/myproject/mychart.
    helm: {}
    repoURL: cr.example.com/myproject # Репозиторий Argo CD из WerfBundle.
    targetRevision: 1.0.0
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - CreateNamespace=true
```

## Публикация артефакта в registry

OCI-чарт Helm требует, чтобы имя чарта в `Chart.yaml` совпадало с последним элементом пути
в OCI-registry. Поэтому название чарта необходимо использовать в названии бандла:

```sh
werf bundle publish --repo cr.example.com/myproject/mychart --tag 1.0.0
```

Подробнее о бандлах — [в документации werf](https://ru.werf.io/documentation/v1.2/advanced/ci_cd/werf_with_argocd/configure_ci_cd.html).

## Автообновление бандла

Argo CD Image Updater используется для автоматического обновления Application из опубликованного
werf-бандла в pull-модели. Image Updater сканирует OCI-репозиторий с заданным интервалом и обновляет
`targetRevision` в Application, посредством чего обновляется все приложение из обновленного
артефакта. Мы используем [измененный Image Updater](https://github.com/argoproj-labs/argocd-image-updater/pull/405), который умеет работать с OCI-registry и werf-бандлами.

### Правила обновлений образов

В Application нужно добавить аннотацию с правилами обновления образа
(подробнее — [в документации werf](https://ru.werf.io/documentation/v1.2/advanced/ci_cd/werf_with_argocd/configure_ci_cd.html#непрерывное-развертывание)).

Пример правила, обновляющего патч-версии приложения (`1.0.*`):

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd-image-updater.argoproj.io/chart-version: ~ 1.0
```

### Настройки доступа к registry

У сервис-аккаунта `argocd-image-updater` есть права на работу с ресурсами только в namespace
`d8-delivery`, поэтому именно в нем необходимо создать Secret с параметрами доступа к registry, на
который ссылается поле `credentials`.

#### Индивидуальная настройка для Application

Сослаться на параметры доступа можно индивидуально в каждом Application с помощью аннотации `argocd-image-updater.argoproj.io/pull-secret`.

Пример:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd-image-updater.argoproj.io/chart-version: ~ 1.0
    argocd-image-updater.argoproj.io/pull-secret: pullsecret:d8-delivery/example-registry
```

## Особенности аутентификации командной утилиты `argocd`

### Пользователь Argo CD

Задайте `username` и `password` в конфигурации Argo CD или используйте пользователя `admin`. Пользователь `admin` по
умолчанию выключен, поэтому его необходимо включить.

Чтобы включить пользователя `admin`:

1. Откройте конфигурацию модуля `delivery`:

   ```sh
   kubectl edit mc delivery
   ```

1. Установите параметр `spec.settings.argocd.admin.enabled` в `true`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: delivery
     # (...)
   spec:
     enabled: true
     settings:
       argocd:
         admin:
           enabled: true
     version: 1
   ```

### kubectl

Если настроен [внешний доступ к Kubernetes API](../../modules/150-user-authn/configuration.html#parameters-publishapi), `argocd`
может использовать запросы к kube-apiserver:

```sh
argocd login argocd.example.com --core
```

Утилита `argocd` [не позволяет указывать namespace](https://github.com/argoproj/argo-cd/issues/9123)
во время вызова и рассчитывает на установленное значение в `kubectl`. Модуль `delivery`
находится в namespace `d8-delivery`, поэтому на время работы с `argocd` нужно выбрать namespace `d8-delivery` для использования по умолчанию.

Выполните следующую команду для выбора namespace `d8-delivery` в качестве namespace по умолчанию:

```sh
kubectl config set-context --current --namespace=d8-delivery
```

### Dex

Авторизация через Dex **не работает для CLI**, но работает в веб-интерфейсе.

Вот так **не работает**, потому нет возможности зарегистрировать публичного клиента для DexClient, в
роли которого выступает Argo CD:

```sh
argocd login argocd.example.com --sso
```

## Частичное использование WerfSource CRD

### Без репозитория Argo CD

WerfSource отвечает за создание репозитория в Argo CD и внесение registry в конфигурацию
Image Updater. От создания репозитория можно отказаться в WerfSource, для этого нужно установить
параметр `spec.argocdRepoEnabled=false`. Это пригодится, если используется тип репозитория,
отличный от OCI, например Chart Museum или git:

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

#### Как самостоятельно создать репозиторий Argo CD для OCI-registry

Registry играет роль репозитория бандлов. Чтобы это работало, нужно в репозитории включить режим
OCI. Однако веб-интерфейс не позволяет установить флаг `enableOCI`, поэтому его нужно добавить вне веб-интерфейса.

##### Argo CD CLI

Утилита `argocd` поддерживает флаг `--enable-oci`:

```sh
$ argocd repo add cr.example.com/myproject \
  --enable-oci \
  --type helm \
  --name REPO_NAME \
  --username USERNAME \
  --password PASSWORD
```

##### Веб-интерфейс и kubectl

В существующий репозиторий недостающий флаг можно добавить вручную:

```sh
kubectl -n d8-delivery edit secret repo-....
```

```yaml
apiVersion: v1
kind: Secret
stringData:           # <----- Добавить
  enableOCI: "true"   # <----- и сохранить.
data:
  # (...)
metadata:
  # (...)
  name: repo-....
  namespace: d8-delivery
type: Opaque
```

### Без registry для Image Updater

Registry в `configmap/argocd-image-updater-config` могут быть настроены только через WerfSource,
потому что `deckhouse` генерирует этот ConfigMap, используя объекты WerfSource. Если этот способ не
подходит, конфигурацию Image Updater можно задать с помощью аннотаций в каждом
Application индивидуально:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd-image-updater.argoproj.io/chart-version: ~ 1.0
    argocd-image-updater.argoproj.io/pull-secret: pullsecret:d8-delivery/example-registry
```
