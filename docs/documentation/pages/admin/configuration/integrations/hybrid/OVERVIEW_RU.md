---
title: Гибридная интеграция
permalink: ru/admin/integrations/hybrid/overview.html
lang: ru
---

Deckhouse Kubernetes Platform (DKP) имеет возможность использовать ресурсы облачных провайдеров для расширения ресурсов статических кластеров. Поддерживаемые интеграция с облаками на базе [OpenStack](../public/openstack/overview.html) и [vSphere](../public/vsphere/vsphere-overview.html).

Гибридный кластер представляет собой объединенные в один кластер bare-metal-узлы и узлы vSphere или OpenStack. Для создания такого кластера необходимо наличие L2-сети между всеми узлами кластера.

## Гибридный кластер с vSphere

Выполните следующие шаги:

1. Удалите flannel из kube-system: `kubectl -n kube-system delete ds flannel-ds`.
2. Настройте интеграцию и пропишите необходимые для работы параметры.

{% alert level="warning" %}
Cloud-controller-manager синхронизирует состояние между vSphere и Kubernetes, удаляя из Kubernetes те узлы, которых нет в vSphere. В гибридном кластере такое поведение не всегда соответствует потребности, поэтому, если узел Kubernetes запущен не с параметром `--cloud-provider=external`, он автоматически игнорируется (Deckhouse прописывает `static://` на узлы в `.spec.providerID`, а cloud-controller-manager такие узлы игнорирует).
{% endalert %}

## Гибридный кластер с OpenStack

Выполните следующие шаги:

1. Удалите flannel из kube-system: `kubectl -n kube-system delete ds flannel-ds`.
2. Настройте интеграцию и пропишите необходимые для работы параметры.
3. Создайте один или несколько custom resource [OpenStackInstanceClass](cr.html#openstackinstanceclass).
4. Создайте один или несколько custom resource [NodeManager](../../modules/node-manager/cr.html#nodegroup) для управления количеством и процессом заказа машин в облаке.

{% alert level="warning" %}
Cloud-controller-manager синхронизирует состояние между OpenStack и Kubernetes, удаляя из Kubernetes те узлы, которых нет в OpenStack. В гибридном кластере такое поведение не всегда соответствует потребности, поэтому, если узел Kubernetes запущен не с параметром `--cloud-provider=external`, он автоматически игнорируется (Deckhouse прописывает `static://` на узлы в `.spec.providerID`, а cloud-controller-manager такие узлы игнорирует).
{% endalert %}

### Подключение storage

Если вам требуются PersistentVolumes на узлах, подключаемых к кластеру из OpenStack, необходимо создать StorageClass с нужным OpenStack volume type. Получить список типов можно с помощью команды `openstack volume type list`.

Например, для volume type `ceph-ssd`:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-ssd
provisioner: csi-cinderplugin # Обязательно должно быть так.
parameters:
  type: ceph-ssd
volumeBindingMode: WaitForFirstConsumer
```
