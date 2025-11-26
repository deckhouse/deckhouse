---
title: Что делать, если обновление DKP висит в статусе Release is suspended?
permalink:
lang: ru
---

<!-- Предлагаю разместить эту информацию куда-нибудь сюда: https://deckhouse.ru/products/kubernetes-platform/documentation/latest/admin/configuration/update/faq.html#%D0%B5%D1%81%D0%BB%D0%B8-%D1%87%D1%82%D0%BE-%D1%82%D0%BE-%D0%BF%D0%BE%D1%88%D0%BB%D0%BE-%D0%BD%D0%B5-%D1%82%D0%B0%D0%BA -->

Состояние релиза `Release is suspended` говорит о том, что он был отложен, и на данный момент недоступен (не рекомендуется) для установки. В таком случае предлагается оставаться на последнем доступном релизе, либо на установленном в данный момент (он будет иметь статус `Deployed`).

Для просмотра списка релизов используйте команду:

```shell
d8 k get deckhousereleases.deckhouse.io
```

Пример вывода:

```console
NAME       PHASE        TRANSITIONTIME   MESSAGE
v1.69.13   Skipped      3h46m
v1.69.14   Skipped      3h46m
v1.69.15   Skipped      3h46m
v1.69.16   Superseded   160m
v1.70.12   Suspended    49d              Release is suspended
v1.70.13   Skipped      36d
v1.70.14   Skipped      34d
v1.70.15   Skipped      28d
v1.70.16   Skipped      19d
v1.70.17   Deployed     160m
v1.71.3    Suspended    14d              Release is suspended
```
