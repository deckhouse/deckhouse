---
title: "Частые вопросы"
permalink: ru/user/managed-services/faq/
description: "Диагностика частых проблем при использовании managed PostgreSQL в Deckhouse Kubernetes Platform."
lang: ru
---

Ниже собраны частые проблемы при использовании managed PostgreSQL и способы их диагностики.

## Postgres не переходит в готовое состояние

Проверьте экземпляры PostgreSQL, события и PVC:

```shell
d8 k get pods -n postgres -o wide
d8 k get events -n postgres --sort-by=.lastTimestamp
d8 k get pvc -n postgres
```

Если `d8 k apply` отклоняет изменение, проверьте сообщение об ошибке и ограничения выбранного PostgresClass.

## Экземпляры PostgreSQL остаются в состоянии Pending

Проверьте события Pod:

```shell
d8 k get pods -n postgres
d8 k describe pod <POD_NAME> -n postgres
```

Где `<POD_NAME>` — имя Pod экземпляра PostgreSQL.

Одной из причин может быть отсутствие узлов, соответствующих правилам размещения выбранного PostgresClass (`nodeSelector`, `nodeAffinity` или `tolerations`). Если подходящих узлов нет, обратитесь к администратору кластера или выберите другой доступный PostgresClass.

## Проблемы со снимками

Проверьте VolumeSnapshotClass и VolumeSnapshot:

```shell
d8 k get volumesnapshotclass
d8 k get volumesnapshot -n postgres
```
