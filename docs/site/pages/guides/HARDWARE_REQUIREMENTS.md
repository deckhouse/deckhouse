---
title: Hardware requirements for bare metal cluster nodes
permalink: en/guides/hardware-requirements.html
description: Hardware requirements for cluster nodes managed by Deckhouse Kubernetes Platform.
lang: en
---

Before deploying a cluster running Deckhouse Kubernetes Platform, you have to decide on the configuration of the future cluster and select parameters for its nodes (e. g., RAM, CPU, etc.).

## Installation Planning

Before deploying a cluster, you need to plan for the resources that you might need to run the cluster. The following questions will help you plan ahead:

* What is the expected load?
* Does your cluster require a high load mode?
* Does your cluster require a high availability mode?
* Which DKP modules do you intend to use?

{% alert level="info" %}
The information below applies to a Deckhouse Kubernetes Platform installation running the [Default module set](/products/kubernetes-platform/documentation/v1/#module-sets).
{% endalert %}

The answers to these questions can help you estimate the number of nodes recommended for your cluster deployment. See [Deployment Scenarios](#deployment-scenarios) to learn more.

## Deployment Scenarios

{% alert level="warning" %}
This section helps you estimate the resources required for the cluster based on the expected load.
{% endalert %}

In general, your cluster might include the following node types:

* **master nodes** — cluster management nodes
* **frontend nodes** — nodes that balance incoming traffic; Ingress controllers run on them
* **monitoring nodes** — these nodes are used to run Grafana, Prometheus and other monitoring tools
* **system nodes** — these nodes are intended to run Deckhouse modules
* **worker nodes** — these nodes are used to run user applications

See [Configuration Features](https://deckhouse.io/products/kubernetes-platform/guides/production.html#things-to-consider-when-configuring) of the "Going to Production" section for details on these node types.

### Minimum Сluster Сonfiguration

Minimum cluster configuration is suitable for small projects with low load and low reliability requirements. Such a cluster includes:

* **master node** — 1 pc
* **worker nodes** — 1 pc or more

Such a configuration requires **8+ CPUs and 16+GB RAM** as well as speedy **400+ IOPS disks** and a minimum of 50GB for the master nodes.

It is up to you to define the characteristics of the worker node based on the expected user load. Note that in this configuration, some of the DKP components will also run on the worker node.

{% alert level="warning" %}
Such a cluster configuration is risky because if a single master node fails, the entire cluster will be affected.
{% endalert %}

### Typical Сluster Сonfiguration

This is the recommended configuration that can tolerate the failure of two master nodes. It greatly improves service availability.
A typical cluster includes:

* **master nodes** — 3 pcs
* **frontend nodes** — 2 pcs
* **system nodes** — 2 pcs
* **worker nodes** — 1+ pcs depending on the expected load

Such a configuration requires **24+ CPUs and 48+GB RAM** as well as speedy **400+ IOPS disks** and a minimum of 50GB for the master nodes.

{% alert level="info" %}
It is recommended to allocate a dedicated [StorageClass](https://deckhouse.io/products/kubernetes-platform/documentation/v1/deckhouse-configure-global.html#parameters-storageclass) on fast disks for Deckhouse components.
{% endalert %}

### High-load Cluster Configuration

Unlike the typical configuration, this configuration includes dedicated monitoring nodes, enabling a high level of observability in the cluster even under high loads.
The cluster with high load includes:

* **master nodes** — 3 pcs
* **frontend nodes** — 2 pcs
* **system nodes** — 2 pcs
* **monitoring nodes** — 2 pcs
* **worker nodes** — 1+ pcs depending on the expected load

Such a configuration requires **28+ CPUs and 64+GB RAM** as well as speedy **400+ IOPS disks** and a minimum of 50GB for the master and monitoring nodes.

It is recommended to allocate a dedicated [StorageClass](https://deckhouse.io/products/kubernetes-platform/documentation/v1/deckhouse-configure-global.html#parameters-storageclass) on fast disks for Deckhouse components.

## Cluster Node Requirements

Each cluster node must comply with the following requirements:

* [supported OS version](https://deckhouse.io/products/kubernetes-platform/documentation/v1/supported_versions.html) (Limitations apply for some operating systems, see the "Notes" section in the supported versions table.)
* Linux kernel version 5.7 or later
* **unique hostname** within the cluster servers (virtual machines)
* access over HTTP(S) to the `registry.deckhouse.io` container image repository or the corporate registry if installed in a private environment
* access to the default package repositories for the operating system in use
* no container runtime packages (e.g., containerd or Docker) must be present on the node
* the `cloud-utils` and `cloud-init` packages must be installed on the node **and their corresponding services must be running**
* all nodes must have access to time servers (NTP) so that the [Chrony](../documentation/v1/modules/chrony/) module can synchronize time

### Minimum Node Resource Requirements

Regardless of the way the cluster will be operated and the workloads expected to be run in it, the following **minimum resources** are recommended for infrastructure nodes depending on their role in the cluster:

* **Master node** — 4 CPUs, 8GB of RAM, 60GB of disk space on performant disk (400+ IOPS)
* **Frontend node** — 2 CPUs, 4GB of RAM, 50GB of disk space
* **Monitoring node** (for high-load clusters) — 4 CPUs, 8GB of RAM, 50GB of disk space on performant disk (400+ IOPS)
* **System node**:
  * 2 CPUs, 4GB of RAM, 50GB of disk space — if there are dedicated monitoring nodes in the cluster
  * 4 CPUs, 8GB of RAM, 60GB of disk space on performant disk (400+ IOPS) — if no dedicated monitoring nodes are running in the cluster
* **Worker node** — the requirements are similar to those for the master node, but largely depend on the nature of the workload running on the node(s).

{% alert level="warning" %}
The way the cluster will run on minimum requirement nodes largely depends on which DKP modules are enabled.

We recommend to increase the node resources if the number of enabled modules is large.
{% endalert %}

### Resource Requirements for Nodes Running Production Workloads

Для того чтобы в production-среде кластер не столкнулся с нехваткой ресурсов на узлах, а все модули в любых конфигурациях могли нормально работать, рекомендуются следующие системные требования для узлов:

* **Мастер-узел** — 8 CPU, 16 ГБ RAM, 60 ГБ дискового пространства на быстром диске (400+ IOPS);
* **Frontend-узел** — 2 CPU, 4 ГБ RAM, 50 ГБ дискового пространства.
* **Узел мониторинга** (для нагруженных кластеров) — 4 CPU, 8 ГБ RAM; 50 ГБ дискового пространства на быстром диске (400+ IOPS).
* **Системный узел**:
  * 6 CPU, 12 ГБ RAM, 50 ГБ дискового пространства — если в кластере есть выделенные узлы мониторинга;
  * 8 CPU, 16 ГБ RAM, 60 ГБ дискового пространства на быстром диске (400+ IOPS) — если в кластере нет выделенных узлов мониторинга.
* **Worker-узел** — 4 CPU, 12 ГБ RAM, 60 ГБ дискового пространства на быстром диске (400+ IOPS).

### Cluster With a Single Master Node

{% alert level="warning" %}
Such a cluster lacks fault tolerance. We highly advise you against using this kind of clusters in production environments.
{% endalert %}

В некоторых случаях может быть достаточно всего одного единственного узла, который будет выполнять все описанные выше роли узлов в одиночку. Например, это может быть полезно в ознакомительных целях или для каких-то совсем простых задач, не требовательных к ресурсам.

В [быстром старте](../gs/bm/step5.html) есть инструкции по развертыванию кластера на единственном master-узле. После снятия taint с узла на нем будут запущены все компоненты кластера, входящие в выбранный bundle модулей (по умолчанию [bundle: Default](../documentation/v1/modules/002-deckhouse/configuration.html#parameters-bundle)). Для успешной работы кластера в таком режиме потребуются 6 CPU, 12 ГБ RAM и 60 ГБ дискового пространства на быстром диске (400+ IOPS). Эта конфигурация позволит запускать некоторую полезную нагрузку, при этом Deckhouse будет занимать примерно ~25% CPU, ~9 ГБ RAM, ~20 ГБ дискового пространства.

В такой конфигурации при нагрузке в 2500 RPS на условное веб-приложение (статическая страница Nginx) из 30 подов и входящем трафике в 24 Мбит/с:

- Нагрузка на CPU суммарно будет повышаться до ~60%.
- Значения RAM и диска не возрастают, но в реальности будут зависеть от кол-ва метрик, собираемых с приложений, и характера обработки полезной нагрузки.

{% alert level="info" %}
Рекомендуется провести нагрузочное тестирование вашего приложения, и с учетом этого скорректировать мощности сервера.
{% endalert %}

## Требования к аппаратным характеристикам узлов

Машины, предназначенные стать узлами будущего кластера, должны соответствовать следующим требованиям:

* **Архитектура ЦП** — на всех узлах должна использоваться архитектура ЦП `x86_64`.
* **Однотипные узлы** — все узлы должны иметь одинаковую конфигурацию для каждого типа узлов. Узлы должны быть одной марки и модели с одинаковой конфигурацией ЦП, памяти и хранилища.
* **Сетевые интерфейсы** — каждый узел должен иметь по крайней мере один сетевой интерфейс для маршрутизируемой сети.

## Требования к сети между узлами

Узлы должны иметь сетевой доступ друг к другу. Между узлами должны соблюдаться [сетевые политики](../documentation/v1/network_security_setup.html).

### Требования к MTU внутри сети

Требований к MTU нет.

### Требования к IP-адресам узлов

У каждого узла должен быть постоянный IP-адрес.

{% alert level="warning" %}
В случае использования DHCP-сервера для распределения IP-адресов по узлам необходимо настроить в нём чёткое соответствие выдаваемых адресов каждому узлу. Смена IP-адреса узлов нежелательна.
{% endalert %}

## Сообщество

{% alert %}
Следите за новостями проекта в [Telegram](https://t.me/deckhouse_ru).
{% endalert %}

Вступите в [сообщество](https://deckhouse.ru/community/about.html), чтобы быть в курсе важных изменений и новостей. Вы сможете общаться с людьми, занятыми общим делом. Это позволит избежать многих типичных проблем.

Команда Deckhouse знает, каких усилий требует организация работы production-кластера в Kubernetes. Мы будем рады, если Deckhouse позволит вам реализовать задуманное. Поделитесь своим опытом и вдохновите других на переход в Kubernetes.
