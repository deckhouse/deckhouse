---
title: "Cloud provider — OpenStack"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью OpenStack."
---

Модуль `cloud-provider-openstack` обеспечивает интеграцию Deckhouse Kubernetes Platform с облаками на базе [OpenStack](https://www.openstack.org/). Он предоставляет возможность модулю [`node-manager`](/modules/node-manager/) использовать ресурсы OpenStack при заказе узлов для [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-openstack`:

- Управление ресурсами OpenStack через `cloud-controller-manager`:
  - актуализирует метаданные серверов OpenStack и узлов Kubernetes и удаляет из Kubernetes узлы, которых больше нет в OpenStack;
  - создаёт балансировщики нагрузки (Octavia) для Service с типом LoadBalancer.
- Заказ блочных дисков через CSI-драйвер Cinder (`cinder.csi.openstack.org`). Manila (filesystem) не поддерживается. CSI-драйвер Cinder поддерживает повторную аутентификацию в OpenStack с обновлением сервисного каталога, что повышает устойчивость длительно работающих подов с томами.
- Заказ базовой инфраструктуры и CloudPermanent-узлов с помощью [Terraform/OpenTofu-провайдера](/products/kubernetes-platform/documentation/v1/architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-openstack.html#взаимодействия-модуля) `terraform-provider-openstack/openstack`.
- Заказ CloudEphemeral-узлов через Machine Controller Manager (MCM) или Cluster API (CAPI). Параметры виртуальных машин задаются в ресурсе [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass).
- Регистрация в модуле [`node-manager`](/modules/node-manager/), чтобы [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass) можно было указывать при описании [NodeGroup](/modules/node-manager/cr.html#nodegroup).
- Автоматическое включение CNI для новых кластеров. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/). Режим сети зависит от параметра [`podNetworkMode`](/modules/cloud-provider-openstack/configuration.html#parameters-podnetworkmode).

{% alert level="warning" %}
Модуль находится в процессе миграции управления CloudEphemeral-узлами с Machine Controller Manager (MCM) на Cluster API (CAPI). Существующие [NodeGroup](/modules/node-manager/cr.html#nodegroup) продолжают использовать MCM, а новые по умолчанию создаются с использованием CAPI. Порядок миграции существующих групп — в разделе [«Как мигрировать группы узлов на Cluster API (CAPI)»](/products/kubernetes-platform/documentation/v1/faq.html#как-мигрировать-группы-узлов-на-cluster-api-capi).
{% endalert %}
