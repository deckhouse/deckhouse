---
title: "Cloud provider — Basis Dynamix: FAQ"
---

## Как настроить LoadBalancer?

Для настройки Service типа LoadBalancer добавьте в манифест Service следующие аннотации:

```yaml
metadata:
  annotations:
    dynamix.cpi.flant.com/internal-network-name: <internal_name>
    dynamix.cpi.flant.com/external-network-name: <external_name>
```

Обе аннотации обязательны:

- `dynamix.cpi.flant.com/internal-network-name` — имя внутренней сети в Basis Dynamix;
- `dynamix.cpi.flant.com/external-network-name` — имя внешней сети в Basis Dynamix.

Термины «внутренняя сеть» и «внешняя сеть» используются в контексте Basis Dynamix. Внешняя сеть не обязательно должна быть публичной и может использовать серые IP-адреса.

Если одна из аннотаций не указана, cloud-controller-manager завершит обработку Service с ошибкой.

## Чем задаётся размещение диска?

Именем storage policy и ничем больше. Имя задаётся в двух местах:

- [`storagePolicy`](cluster_configuration.html#dynamixclusterconfiguration-storagepolicy) в корне DynamixClusterConfiguration — политика всех виртуальных машин кластера;
- `storagePolicy` в instanceClass — в [`masterNodeGroup.instanceClass`](cluster_configuration.html#dynamixclusterconfiguration-masternodegroup-instanceclass-storagepolicy), в [`nodeGroups[].instanceClass`](cluster_configuration.html#dynamixclusterconfiguration-nodegroups-instanceclass-storagepolicy) или в [DynamixInstanceClass](cr.html#dynamixinstanceclass-v1-spec-storagepolicy) — переопределяет общее значение только для этой группы узлов.

Storage policy описывает набор доступных аккаунту пар storage endpoint + пул и лимит IOPS. На какую из этих пар попадёт конкретный диск, решает Basis Dynamix, а не Deckhouse: модуль называет политику и оставляет размещение платформе. Параметра, которым можно выбрать storage endpoint или пул, нет, и модуль не переносит уже созданный диск в другое размещение.

По той же причине смена storage policy — общей для кластера или в отдельном instanceClass — пересоздаёт узлы типа CloudEphemeral, к которым она относится.

## Какие StorageClass'ы создаёт модуль?

По одному на каждую доступную аккаунту кластера storage policy со статусом `ENABLED`, с именем по имени политики. Имя политики, не подходящее под имя объекта Kubernetes, приводится к допустимому, поэтому две политики могут претендовать на одно имя StorageClass'а — в этом случае класс получит только одна из них.

Чтобы часть из них не появлялась в кластере, перечислите имена или регулярные выражения в параметре [`storageClass.exclude`](configuration.html#parameters-storageclass-exclude).
