---
title: Virtualization API
permalink: en/architecture/virtualization/api.html
search: virtualization controller, virtualization api
description: Architecture of the Virtualization API component of virtualization module in Deckhouse Kubernetes Platform.
---

The Virtualization API component of [`virtualization`](/modules/virtualization/) module manages custom resources of the following API groups:

1. `virtualization.deckhouse.io`: The main group, it includes the following custom resources:

   - VirtualMachine: A resource that describes the virtual machine (VM) configuration and status.
   - VirtualMachineClass: A resource that describes a set of parameters for VirtualMachine resources, such as CPU and RAM specification, NodeSelector and Tolerations.
   - VirtualDisk: A resource that describes desired VM disk configuration.
   - VirtualImage: A resource that describes VM disk image, which can be used as a data source for new VirtualDisks resources, or an installation image (ISO) that can be mounted directly into a VirtualMachine resource.

   The full list of the main API group resources is given [in the module documentation](/modules/virtualization/cr.html).

   Virtualization-сontroller manages the resources of the main group.

2. `subresources.virtualization.deckhouse.io`: Subresources group. Subresources are additional operations or actions that can be performed on core resources (for example, VirtualMachine) via the Kubernetes API. They provide interfaces for managing specific aspects of resources without affecting the entire object. Instead of the declarative resource familiar to Kubernetes, they are endpoints for imperative operations. The group includes the following subresources:
   
   - virtualmachines/console;
   - virtualmachines/vnc;
   - virtualmachines/portforward;
   - virtualmachines/addvolume;
   - virtualmachines/removevolume;
   - virtualmachines/freeze;
   - virtualmachines/unfreeze;
   - virtualmachines/addresourceclaim;
   - virtualmachines/removeresourceclaim.

   Virtualization-api manages subresources.

Virtualization API component of the module uses KubeVirt custom resources as a backend to manage VMs, VM disks and images.

модуля для управления ВМ, дисками и образами ВМ использует в качестве бэкенда кастомные ресурсы KubeVirt. [KubeVirt](https://github.com/kubevirt/kubevirt) is an open-source project that allows you to launch, deploy, and manage VMs using Kubernetes as an orchestration platform. It enables a cooperation between traditional VMs and container workloads in the same Kubernetes cluster, providing a single control plane.

## Архитектура Virtualization API

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

- На схеме контейнеры разных подов показаны как взаимодействующие напрямую. Фактически обмен выполняется через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса приводится над стрелкой.
- Поды могут быть запущены в нескольких репликах, однако на схеме каждый под показан в единственном экземпляре.
{% endalert %}

Архитектура компонента Virtualization API модуля [`virtualization`](/modules/virtualization/) на уровне 2 модели C4 и его взаимодействия с другими компонентами DKP изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура компонента Virtualization API модуля virtualization](../../../images/architecture/virtualization/c4-l2-virtualization-controller.ru.png)

## Компоненты Virtualization API

Virtualization API состоит из следующих компонентов:

1. **Virtualization-api** — [Kubernetes Extension API Server](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-extension-api-server/), обслуживающий запросы к API-группе `subresources.virtualization.deckhouse.io`. В качестве бэкенда virtualization-api использует сабресурсы из API-группы `subresources.kubevirt.io`. Virtualization-api обращается напрямую на эндпойнт компонента virt-api, который является [Kubernetes Extension API Server](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-extension-api-server/), обслуживающий запросы к аналогичным сабресурсам из API-группы `subresources.kubevirt.io`.

   Состоит из одного контейнера:

   - **virtualization-api**.

2. **Virtualization-controller** — контроллер, выполняющий следующие операции:

   - управление кастомными ресурсами основной API-группы `virtualization.deckhouse.io`.Virtualization-controller ограничен в изменении большей части лейблов, аннотаций и атрибутов спецификации ресурсов. Virtualization-controller разрешено вносить следующие изменения в кастомные ресурсы:

     - добавление и удаление finalizers в атрибуте `metadata.finalizers`;
     - добавление и удаление owners в атрибуте `metadata.ownerReferences`;
     - изменение статуса ресурса.

     В качестве бэкенда virtualization-controller использует кастомные ресурсы из API-группы `kubevirt.io`.
   
   - валидация ресурсов из API-группы `virtualization.deckhouse.io` с помощью механизма [Validating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/)
   
   - запуск подов dvcr-importer и dvcr-uploader для выполнения сценариев импорта и загрузки дисков и образов ВМ в хранилище образов DVCR. [DVCR (или Deckhouse Virtualization Container Registry)](dvcr.html) — специализированный реестр для хранения и кеширования образов ВМ.

   - выполнение операций над виртуальными машинами посредством запросов к некоторым сабресурсам API-группы `subresources.virtualization.deckhouse.io`, например, `virtualmachines/freeze` и `virtualmachines/unfreeze`.

   Компонент содержит следующие контейнеры:

      - **virtualization-controller** — основной контейнер, реализующий контроллер и вебхук-сервер;
      - **proxy** (он же **kube-api-rewriter**) —  сайдкар-контейнер, выполняющий модификацию проходящих через него запросов API, а именно переименование метаданных кастомных ресурсов. Это необходимо, поскольку компоненты Kubevirt используют API-группы вида `*.kubevirt.io`, а другие компоненты модуля [`virtualization`](/modules/virtualization/) используют аналогичные ресурсы, но с API-группой вида `*.virtualization.deckhouse.io`. Kube-api-rewriter является шлюзом, проксирующим запросы между контроллерами, управляющими ресурсами из разных API-групп. Является [Open Source-проектом](https://github.com/deckhouse/kube-api-rewriter);
      - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам контроллера и сайдкар-контейнера proxy. Является [Open Source-проектом](https://github.com/brancz/kube-rbac-proxy).

## Взаимодействия компонента Virtualization API

Virtualization-api взаимодействует со следующими компонентами:

1. **Kube-apiserver** — читает список кастомных ресурсов VirtualMachine, которые нужны для обработки запросов к сабресурсам.
1. **Virt-api** — отправляет запросы к сабресурсам KubeVirt. Запросы проходят через аналогичный сайдкар-контейнер **proxy**, который переименовывает метаданные из API-группы `subresources.virtualization.deckhouse.io` в API-группу `subresources.kubevirt.io` и проксирует их на эндпойнт virt-api (Kubernetes Extension API Server KubeVirt).

Virtualization-controller взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

   - отправляет измененные [кастомные ресурсы модуля virtualization](/modules/virtualization/cr.html) через сайдкар-контейнер proxy, который переименовывает метаданные из API-группы `internal.virtualization.deckhouse.io` в API-группу `kubevirt.io`;
   - выполняет авторизацию запросов на получение метрик.

С Virtualization API взаимодействуют следующие внешние компоненты:

1. **Kube-apiserver**:

   - пересылает запросы к сабресурсам API-группы `subresources.virtualization.deckhouse.io`;
   - отправляет запросы на валидацию ресурсов API-группы `virtualization.deckhouse.io`.

1. **Prometheus-main** — собирает метрики компонентов.
