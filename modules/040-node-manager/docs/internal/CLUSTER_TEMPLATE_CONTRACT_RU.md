---
title: "Контракт cluster-template v1 для CAPI"
description: Как модуль cloud-провайдера описывает инфраструктурный кластер и Secret с учётными данными для node-controller.
---

Это полное описание файлов `capi/cluster.yaml` и `capi/credentials.yaml`, которые поставляет
модуль cloud-провайдера. Второй файл необязателен. **Чего нет в этом документе — того нет в
контракте.** Поднимайте `version` только при несовместимом изменении. Для нового поля укажите
релиз Deckhouse, начиная с которого оно доступно.

Для шаблонов машин действует та же модель данных и та же песочница. Она описана в
[`MACHINE_TEMPLATE_CONTRACT_RU.md`](MACHINE_TEMPLATE_CONTRACT_RU.md).

## Что node-controller делает с файлами

Модуль провайдера кладёт файлы в Secret `d8-cloud-provider-<type>-capi` в `kube-system`.
ClusterReconciler читает и проверяет оба шаблона до применения любого из объектов. Общие ресурсы
CAPI — `Cluster`, `DeckhouseControlPlane` и `MachineHealthCheck` — он создаёт сам.

Ресурсы провайдера обновляются через server-side apply. Если в существующем объекте уже есть
`spec.controlPlaneEndpoint`, node-controller сохраняет его. Некоторые CAPI-провайдеры разрешают
задать это поле только один раз, а найденные адреса control plane со временем могут измениться.

## Структура файлов

Оба файла имеют одну и ту же оболочку:

```yaml
version: v1
template: |
  apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
  kind: OpenStackCluster
  metadata:
    name: {{ .cluster.name | quote }}
    namespace: {{ .cluster.namespace | quote }}
  spec: {}
```

Неизвестное поле оболочки, неподдерживаемая версия, пустой или синтаксически неверный шаблон
останавливают обработку с ошибкой.

`cluster.yaml` должен собрать ровно один Kubernetes-объект. Его `apiVersion`, `kind` и
`metadata.name` должны совпасть с `capiClusterAPIVersion`, `capiClusterKind` и `capiClusterName`
из регистрационного Secret провайдера.

Необязательный `credentials.yaml` должен собрать ровно один `v1/Secret`. Оба объекта должны
находиться в `d8-cloud-instance-manager`. Если namespace не указан, node-controller добавит его.
Если `credentials.yaml` удалён или начинает создавать Secret с другим именем, ранее созданный по
этому шаблону Secret удаляется.

## Данные шаблона

### `cluster.yaml`

| Корень | Тип | Доступно с | Что это |
|---|---|---|---|
| `.provider` | map | v1 | Поддерево провайдера из `d8-node-manager-cloud-provider`. |
| `.cluster` | map | v1 | Общие данные кластера из таблицы ниже. |
| `.prefix` | string | v1 | Текущий префикс кластера. Может быть пустым. |
| `.controlPlane.endpoints` | list | v1 | Найденные адреса API в виде `{host, port}`. Если адресов нет, корня `.controlPlane` тоже нет. |

### `credentials.yaml`

Этому файлу доступны только `.provider` и `.cluster`. Он не видит `.prefix` и `.controlPlane`.

### Поля `.cluster`

| Поле | Тип | Доступно с | Что это |
|---|---|---|---|
| `.cluster.name` | string | v1 | Имя CAPI Cluster. |
| `.cluster.namespace` | string | v1 | Namespace для ресурсов CAPI. |
| `.cluster.uuid` | string | v1 | UUID кластера Deckhouse. |
| `.cluster.podSubnet` | string | v1 | CIDR pod-сети. |

Ни `.Values`, ни `.Chart`, ни `.Files`, ни `.Release` здесь нет. Провайдер публикует нужные ему
настройки в своём поддереве `d8-node-manager-cloud-provider`, а node-controller добавляет общие
данные кластера.

## Правила шаблонов

1. В каждом файле должен быть один YAML-документ. Учётные данные кладите в
   `credentials.yaml`, а не вторым документом в `cluster.yaml`.
2. Отсутствующее поле считается ошибкой. Для действительно необязательных ключей используйте
   `get` или `hasKey`.
3. Результат должен зависеть только от входных данных. В песочнице нет часов, случайных значений,
   окружения хоста, доступа к сети, генерации ключей и helm-функций `include`, `tpl`, `lookup`.
4. Метки провайдерского контроллера, например `app`, оставляйте в шаблоне. Общие метки
   `heritage`, `module` и аннотацию `helm.sh/resource-policy: keep` добавляет node-controller.

## Передача ресурсов от Helm

Перед обновлением миграционный хук добавляет существующим ресурсам
`helm.sh/resource-policy: keep`, поэтому Helm не удаляет их вместе со старым шаблоном.
ClusterReconciler применяет тот же объект от имени `node-controller`, удаляет метки владения
Helm и сохраняет UID. Метаданные провайдера и аннотация `keep` остаются.
