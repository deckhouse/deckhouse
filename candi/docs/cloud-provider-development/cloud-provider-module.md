# Модуль 030-cloud-provider-<cloud-provider-name>

Модуль cloud-provider это модуль addon-operator, который представляет собой Helm-чарт и состоит из:

1. Шаблонов ресурсов для Cluster API в каталоге `capi`;
   > Типичный сетап, в случае с провайдером Sample,
   > состоит из шаблонов `SampleCluster` и `MachineTemplate` для `SampleMachine`

2. Ссылки `candi`, которая будет вести на каталог с ресурсами провайдера в `/deckhouse/candi/cloud-providers`, созданный в предыдущих шагах.

3. Самого Helm-chart `Chart.yaml` и директории `charts`, ссылающейся на `deckhouse/helm_lib`.
   Имя чарта совпадает с именем модуля (`cloud-provider-sample`), версия не имеет значения;
   > Важно прописать все что не относится непосредственно к helm в .helmignore.

4. Набора необходимых для работы провайдера ресурсов CRD в каталоге `crds`
   (ресурсы внешних компонентов, таких как Cluster API Provider, хранятся в подкаталоге `external`)

5. Каталога `hooks` с хуками для addon-operator.
6. Каталога `images` где в подкаталогах хранятся сценарии сборки всех используемых в cloud-provider сторонних образов контейнеров.
   Все сценарии сборки должны описываться в виде файлов `werf.inc.yaml` в формате Stapel.
   Иногда мы так же можем разместить сами исходники, из которых собирается образ, в таком подкаталоге.
7. Каталога `openapi`, описывающего структуру данных с которой будет работать модуль внутри helm-шаблонов (Helm Values).
8. Каталога `templates`, в котором хранятся helm-шаблоны манифестов ресурсов используемых модулем в работе.
   Для каждого из компонентов модуля (например, образов из `images`) желательно выделить отдельный подкаталог со специфичными для него ресурсами.

   Так же здесь хранятся описания объектов Secret, используемых модулем 040-node-manager, через которые cloud-provider передает в node-manager информацию о себе.
   (`registration.yaml`, `cni.yaml`).
9. Файла `.namespace`, содержащего имя k8s namespace в котором будут размещаться ресурсы этого провайдера.
   Имя формируется по шаблону `d8-cloud-provider-*****`

>Новый провайдер обязательно нужно добавить в список провайдеров в файле ```/deckhouse/global-hooks/enable_cloud_provider.go```

## Ресурсы Cluster API (`capi`)

Ресурсы, используемые для работы с Cluster API, в случае с провайдером Sample, со стороны модуля состоят из объектов `SampleCluster` и `MachineTemplate`.
Шаблоны этих объектов представляют собой Helm-манифесты.

Содержимое этих объектов определяется выбранным Cluster API Provider, информация о кластерах и машинах попадает в шаблон из InstanceCLass и ProviderClusterConfiguration через helm values.

## Структура Helm Values (`openapi`)

Необходимо описать схему для providerClusterConfiguration, включающую в себя информацию о masterNodeGroup, nodeGroups, provider. Схему для providerDiscoveryData и storageClasses. Это основной набор данных.

## Шаблоны манифестов модуля (`templates`)

* `namespace.yaml` - содержит описание namespace, в котором будут размещаться ресурсы провайдера.
* `registration.yaml` - содержит информацию о провайдере, которая передается в node-manager.
* `registry-secret.yaml` - содержит информацию о docker registry.
* `secret.yaml` - содержит информацию для доступа к API облачного провайдера.

Пример реализации шаблонов можно найти в отдельной ветке репозитория Deckhouse - [cloud-provider-sample](https://github.com/deckhouse/deckhouse/tree/cloud-provider-sample/ee/modules/030-cloud-provider-sample).

## Hooks

Обязательные hooks:

1. Сохранение в values информации от CloudDiscoveryData
1. Регистрация CRDs
1. Регистрация InstanceClass
1. Сохранение в values ClusterConfiguration
1. Hook wait_for_all_master_nodes_to_become_initialized.go одинаковый для всех провайдеров

Пример реализации hooks можно найти в отдельной ветке репозитория Deckhouse - [cloud-provider-sample](https://github.com/deckhouse/deckhouse/tree/cloud-provider-sample/ee/modules/030-cloud-provider-sample).
