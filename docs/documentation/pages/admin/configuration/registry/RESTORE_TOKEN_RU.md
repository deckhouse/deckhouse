---
title: Восстановление подключения к registry при истекшем и неверном лицензионном токене
permalink: ru/admin/configuration/registry/restore-token.html
description: "Восстановление подключения к registry в Deckhouse Kubernetes Platform при проблемах с лицензионным токеном. Восстановление доступа к registry."
lang: ru
---

{% alert level="warning" %}
Используйте эту инструкцию только если недоступна штатная [процедура смены registry](./third-party.html).
{% endalert %}

Если после истечения лицензионного токена поды Deckhouse Kubernetes Platform (DKP) были перезапущены, в их логах появится ошибка подключения к registry при загрузке образов DKP. Чтобы переключить кластер на новый токен, на любом master-узле выполните следующие шаги:

1. Загрузите текущую конфигурацию секрета `deckhouse-registry` во временный файл:

   ```shell
   d8 k -n d8-system get secret deckhouse-registry -o yaml > /tmp/deckhouse-registry.yaml
   ```

1. Во временном файле `/tmp/deckhouse-registry.yaml` замените значение поля `.dockerconfigjson` на Base64-кодированную строку с параметрами подключения к registry. Получить нужную строку можно командами ниже, подставив свои значения `MYPASSWORD` и `MYREGISTRY`:

   ```shell
   declare MYUSER='license-token'
   declare MYPASSWORD='example-token'
   declare MYREGISTRY='example-regsitry.deckhouse.ru'
   MYAUTH=$(echo -n "$MYUSER:$MYPASSWORD" | base64 -w0)
   MYRESULTSTRING=$(echo -n "{\"auths\":{\"$MYREGISTRY\":{\"username\":\"$MYUSER\",\"password\":\"$MYPASSWORD\",\"auth\":\"$MYAUTH\"}}}" | base64 -w0)
   echo "$MYRESULTSTRING"
   ```

1. Разрешите изменение устаревшего секрета:

   ```shell
   d8 k delete validatingadmissionpolicybindings.admissionregistration.k8s.io heritage-label-objects.deckhouse.io
   ```

1. Импортируйте обновленную конфигурацию:

   ```shell
   d8 k -n d8-system apply -f /tmp/deckhouse-registry.yaml
   ```

1. Найдите проблемный под `deckhouse` на текущем master-узле и удалите его:

   ```shell
   d8 k get pods -n d8-system -o wide
   d8 k delete pod -n d8-system -o deckhouse-<id>
   ```

1. Убедитесь, что новый под `deckhouse` запустился корректно:

   ```shell
   d8 k get pods -n d8-system
   ```

1. При необходимости удалите остальные поды `deckhouse`, находящиеся в некорректном статусе.

1. Повторите [штатную процедуру](./third-party.html) смены registry, подставив ваш токен и нужный адрес registry и редакцию вместо `example`:

   ```shell
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user licence-token --password MY-PASSWORD registry-example.deckhouse.ru/deckhouse/example
   ```
