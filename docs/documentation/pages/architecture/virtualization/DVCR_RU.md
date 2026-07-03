---
title: Deckhouse Virtualization Container Registry (DVCR)
permalink: ru/architecture/virtualization/dvcr.html
lang: ru
search: deckhouse virtualization container registry, dvcr 
description: Архитектура компонента DVCR модуля virtualization в Deckhouse Kubernetes Platform.
---

Компонент Deckhouse Virtualization Container Registry (DVCR) модуля [`virtualization`](/modules/virtualization/) — это специализированное хранилище образов контейнеров для хранения и кеширования образов виртуальных машин (ВМ). Компонент [CDI](cdi.html) модуля [`virtualization`](/modules/virtualization/) использует хранящиеся в DVCR образы в качестве источника для ресурсов InternalVirtualizationDataVolume, с помощью которых создаются диски ВМ, управляемых KubeVirt.

## Архитектура DVCR

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

- На схеме контейнеры разных подов показаны как взаимодействующие напрямую. Фактически обмен выполняется через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса приводится над стрелкой.
- Поды могут быть запущены в нескольких репликах, однако на схеме каждый под показан в единственном экземпляре.
{% endalert %}

Архитектура компонента DVCR модуля [`virtualization`](/modules/virtualization/) на уровне 2 модели C4 и его взаимодействия с другими компонентами DKP изображены на следующей диаграмме:

<!--- Source: structurizr code --->
![Архитектура компонента DVCR модуля virtualization](../../images/architecture/virtualization/c4-l2-virtualization-dvcr.ru.png)

## Компоненты DVCR

DVCR состоит из следующих компонентов:

1. **Dvcr** — хранилище образов на базе [Distribution](https://github.com/distribution/distribution). Distribution — это Open Source-проект, который является основой для хранения и распределения контейнерных образов и другого контента с использованием спецификации [OCI Distribution Specification](https://github.com/opencontainers/distribution-spec). Dvcr используется для хранения и кеширования образов ВМ.

   Компонент содержит следующие контейнеры:

   - **dvcr** — основной контейнер;
   - **dvcr-garbage-collection** — сайдкар-контейнер, выполняющий периодическое удаление образов, у которых нет соответствующих ресурсов в кластере;
   - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам контейнера dvcr. Является [Open Source-проектом](https://github.com/brancz/kube-rbac-proxy).

1. **Dvcr-importer** — *временный* под, состоящий из одного контейнера и запускаемый virtualization-controller для реализации различных сценариев импорта образов и дисков ВМ, таких как:

   - импорт диска или образа ВМ из внешних источников (HTTP-источник, доступный по URL-ссылке, или хранилище образов) в хранилище DVCR;
   - импорт образа ВМ из внешних источников (HTTP-источник, доступный по URL-ссылке, или хранилище образов) в PVC-том. Dvcr-importer не импортирует диск напрямую в PVC. Он загружает источник в хранилище DVCR. Далее создаётся ресурс InternalVirtualizationDataVolume, и [CDI](cdi.html) уже импортирует образ из хранилища DVCR в PVC;
   - импорт образа ВМ, диска или снимка из ресурсов VirtualImage, ClusterVirtualImage, VirtualDisk или VirtualDiskSnapshot в хранилище DVCR.

1. **Dvcr-uploader** — *временный* под, состоящий из одного контейнера и запускаемый virtualization-controller для реализации следующих сценариев загрузки *пользователем* образов и дисков ВМ:

   - загрузка в хранилище DVCR;
   - загрузка в PVC-том. Dvcr-uploader не загружает диск напрямую в PVC. Он загружает источник в хранилище DVCR. Далее создаётся ресурс InternalVirtualizationDataVolume, и [CDI](cdi.html) уже импортирует образ из хранилища DVCR в PVC.

## Взаимодействия DVCR

DVCR взаимодействует со следующими компонентами:

1. **Kube-apiserver** — выполняет `get`/`list`/`watch`-запросы ресурсов VirtualImages, ClusterVirtualImages и VirtualDisks для очистки неиспользуемых образов и координации.
1. **Внешние источники дисков или образов ВМ** — читает диски или образы ВМ при реализации некоторых сценариев импорта в хранилище DVCR.

С DVCR взаимодействуют следующие внешние компоненты:

1. **Virtualization-controller** — запускает поды dvcr-importer и dvcr-uploader для выполнения сценариев импорта и загрузки дисков и образов ВМ.
1. **Ingress-controller** — пересылает запросы пользователя на загрузку диска или образа ВМ в хранилище DVCR через HTTP-эндпоинт сервиса dvcr-uploader.
1. **Cdi-importer** — использует хранящиеся в DVCR образы в качестве источника для ресурсов InternalVirtualizationDataVolume.
