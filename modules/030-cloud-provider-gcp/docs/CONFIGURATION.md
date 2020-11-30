---
title: "Сloud provider — GCP: настройки"
---

Модуль настраивается автоматически исходя из выбранной схемы размещения (custom resource `GCPClusterConfiguration`). В большинстве случаев нет необходимости ручной конфигурации модуля.

Количество и параметры процесса заказа машин в облаке настраиваются в custom resource [`NodeGroup`](/modules/040-node-manager/cr.html#nodegroup) модуля node-manager, в котором также указывается название используемого для этой группы узлов instance-класса (параметр `cloudInstances.classReference` NodeGroup).  Instance-класс для cloud-провайдера GCP — это custom resource [`GCPInstanceClass`](cr.html#gcpinstanceclass), в котором указываются конкретные параметры самих машин.

## Параметры

> **Внимание!** При изменении конфигурационных параметров приведенных в этой секции (параметров, указываемых в ConfigMap deckhouse) **перекат существующих Machines НЕ производится** (новые Machines будут создаваться с новыми параметрами). Перекат происходит только при изменении параметров `NodeGroup` и `GCPInstanceClass`. См. подробнее в документации модуля [node-manager](/modules/040-node-manager/faq.html#как-перекатить-эфемерные-машины-в-облаке-с-новой-конфигурацией).

* `networkName` — имя VPC network в GCP, где будут заказываться instances.
* `subnetworkName` — имя subnet в VPC netwok `networkName`, где будут заказываться instances.
* `region` — имя GCP региона, в котором будут заказываться instances.
* `zones` — Список зон из `region`, где будут заказываться instances. Является значением по умолчанию для поля zones в [NodeGroup](/modules/040-node-manager/cr.html#nodegroup) объекте.
    * Формат — массив строк.
* `extraInstanceTags` — Список дополнительных GCP tags, которые будут установлены на заказанные instances. Позволяют прикрепить к создаваемым instances различные firewall правила в GCP.
    * Формат — массив строк.
    * Опциональный параметр.
* `sshKey` — публичный SSH ключ.
    * Формат — строка, как из `~/.ssh/id_rsa.pub`.
* `serviceAccountJSON` — ключ к Service Account'у с правами Project Admin.
    * Формат — строка c JSON.
    * [Как получить](https://cloud.google.com/iam/docs/creating-managing-service-account-keys#creating_service_account_keys).
* `disableExternalIP` — прикреплять ли внешний IPv4-адрес к заказанным instances. Если выставлен `true`, то необходимо создать [Cloud NAT](https://cloud.google.com/nat/docs/overview) в GCP.
    * Формат — bool. Опциональный параметр.
    * По умолчанию `true`.

## Storage

Модуль автоматически создаёт StorageClasses, покрывающие все варианты дисков в GCP: 

| Тип | Репликация | Имя StorageClass |
|---|---|---|
| standard | none | pd-standard-not-replicated |
| standard | regional | pd-standard-replicated |
| ssd | none | pd-ssd-not-replicated |
| ssd | regional | pd-ssd-replicated |

А также позволяет отфильтровать ненужные StorageClass, указанием их в параметре `exclude`.

* `exclude` — полные имена (или regex выражения имён) StorageClass, которые не будут созданы в кластере.
  * Формат — массив строк.
  * Опциональный параметр.
* `default` — имя StorageClass, который будет использоваться в кластере по умолчанию.
  * Формат — строка.
  * Опциональный параметр.
  * Если параметр не задан, фактическим StorageClass по умолчанию будет либо: 
    * Присутствующий в кластере произвольный StorageClass с default аннотацией.
    * Первый StorageClass из создаваемых модулем (в порядке из таблицы выше).

```yaml
cloudProviderGcp: |
  storageClass:
    exclude: 
    - "pd-standard.*"
    - pd-ssd-replicated
    default: pd-ssd-not-replicated
```
