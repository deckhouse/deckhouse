---
title: "Проверка хеш суммы образа"
description:
---

## Описание

Для проверки целостности образа используется контрольная сумма расчитанная по алгоритму Стрибог (ГОСТ Р 34.11-2012)
Чтобы устанавлеваемые образы проверялись, необнодимо добавить метку ```gost-integrity-controller.deckhouse.io/gost-digest-validation-enabled: true``` в пространство имен кластера где необходимо производить контроль целостности образа.

Пример:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  labels:
    gost-integrity-controller.deckhouse.io/gost-digest-validation-enabled: "true"
  name: default
```

В случае если во время проверки контрольная сумма образа будет некорректная, будет отказано в установке образа, о чем вы получите сообщение.

Если образ находится в закрытом репозитории, для авторизации необходимо указать в спецификации контейнера параметр ```imagePullSecrets```. И создать секрет с данными для авторизации. Подробней можно почитать в [документации](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/).

## Алгоритм расчета контрольной суммы

Для расчета контрольной суммы берется список контрольных сумм слоев образа.  Список сортируется в порядке возрастания и склеивается в одну строку. Затем производится расчет контрольной суммы от этой строки по алгоритму Стрибог (ГОСТ Р 34.11-2012).

Пример расчета контрольной суммы образа nginx:1.25.2:

```text
Контрольные суммы слоев отсортированные в порядке возрастания
[
    "sha256:27e923fb52d31d7e3bdade76ab9a8056f94dd4bc89179d1c242c0e58592b4d5c",
    "sha256:360eba32fa65016e0d558c6af176db31a202e9a6071666f9b629cb8ba6ccedf0",
    "sha256:72de7d1ce3a476d2652e24f098d571a6796524d64fb34602a90631ed71c4f7ce",
    "sha256:907d1bb4e9312e4bfeabf4115ef8592c77c3ddabcfddb0e6250f90ca1df414fe",
    "sha256:94f34d60e454ca21cf8e5b6ca1f401fcb2583d09281acb1b0de872dba2d36f34",
    "sha256:c5903f3678a7dec453012f84a7d04f6407129240f12a8ebc2cb7df4a06a08c4f",
    "sha256:e42dcfe1730ba17b27138ea21c0ab43785e4fdbea1ee753a1f70923a9c0cc9b8"
]

Склеенная строка из контрольных сумм
"sha256:27e923fb52d31d7e3bdade76ab9a8056f94dd4bc89179d1c242c0e58592b4d5csha256:360eba32fa65016e0d558c6af176db31a202e9a6071666f9b629cb8ba6ccedf0sha256:72de7d1ce3a476d2652e24f098d571a6796524d64fb34602a90631ed71c4f7cesha256:907d1bb4e9312e4bfeabf4115ef8592c77c3ddabcfddb0e6250f90ca1df414fesha256:94f34d60e454ca21cf8e5b6ca1f401fcb2583d09281acb1b0de872dba2d36f34sha256:c5903f3678a7dec453012f84a7d04f6407129240f12a8ebc2cb7df4a06a08c4fsha256:e42dcfe1730ba17b27138ea21c0ab43785e4fdbea1ee753a1f70923a9c0cc9b8"

Контрольная сумма образа

2f538c22adbdb2ca8749cdafc27e94baed8645c69d4f0745fc8889f0e1f5a3f9
```

Контрольную сумму в образ можно добавить используя утилиту crane

```bash
crane mutate --annotation gost-digest=1aa84f6d91cc080fe198da7a6de03ca245aea0a8066a6b4fb5a93e40ebec2937 <образ>
```

Для расчета, добавления и проверки контрольной суммы образа можно использовать утилиту gost-image-digest <https://github.com/deckhouse/gost-image-digest>.

Расчет контрольной суммы

```bash
imagedigest calculate nginx:1.25.2
1:14PM INF GOST Image Digest: 2f538c22adbdb2ca8749cdafc27e94baed8645c69d4f0745fc8889f0e1f5a3f9
```

Расчет контрольной суммы с последующим добавлением в метаданные образа и сохранением в репозитории.

```bash
imagedigest add alekseysu/simple-http:v0.2
1:19PM INF GOST Image Digest: 1aa84f6d91cc080fe198da7a6de03ca245aea0a8066a6b4fb5a93e40ebec2937
1:19PM INF Added successfully
```

Проверка контрольной суммы

```bash
imagedigest validate alekseysu/simple-http:v0.2
2:08PM INF GOST Image Digest from image 1aa84f6d91cc080fe198da7a6de03ca245aea0a8066a6b4fb5a93e40ebec2937
2:08PM INF Calculated GOST Image Digest 1aa84f6d91cc080fe198da7a6de03ca245aea0a8066a6b4fb5a93e40ebec2937
2:08PM INF Validate successfully
```
