---
title: Восстановление подключения к Registry при истекшем и неверном лицензионном токене
permalink: ru/admin/configuration/registry/restore-token.html
lang: ru
---

{% alert level="warning" %}
Используйте при недоступности штатной [процедуры смены Registry](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/admin/configuration/registry/third-party.html)
{% endalert %}

Если после истечения лицензионного токена перезапускались поды Deckhouse, то вы увидите в их логах ошибку подключения к Registry для загрузки образов Deckhouse. Для переключения кластера на использование нового токена с любой мастер-ноды выполните эти действия:

1. Загрузите текущую конфигурацию секрета `deckhouse-registry` во временный файл:

   ```shell
   kubectl -n d8-system get secret deckhouse-registry -o yaml > /tmp/deckhouse-registry.yaml
   ```

2. Измените во временном файле `/tmp/deckhouse-registry.yaml` измените значение строки `.dockerconfigjson` на base64-кодированные параметры подключения к Registry. Создать необходимую строку можно командами ниже, заменив значения `MYPASSWORD` и `MYREGISTRY`:

   ```shell
   declare MYUSER='license-token'
   declare MYPASSWORD='example-token'
   declare MYREGISTRY='example-regsitry.deckhouse.ru'
   MYAUTH=$(echo -n "$MYUSER:$MYPASSWORD" | base64 -w0)
   MYRESULTSTRING=$(echo -n "{\"auths\":{\"$MYREGISTRY\":{\"username\":\"$MYUSER\",\"password\":\"$MYPASSWORD\",\"auth\":\"$MYAUTH\"}}}" | base64 -w0)
   echo "$MYRESULTSTRING"
   ```

3. Разрешите изменения устаревшего секрета:
   ```shell
   kubectl delete validatingadmissionpolicybindings.admissionregistration.k8s.io heritage-label-objects.deckhouse.io
   ```
4. Импортируйте обновленную конфигурацию:
   ```shell
   kubectl -n d8-system apply -f /tmp/deckhouse-registry.yaml
   ```

5. Проверьте наличие проблемного пода `deckhouse` на текущей мастер-ноде, удалите проблемный под `deckhouse`:
   ```shell
   kubectl get pods -n d8-system -o wide
   kubectl delete pod -n d8-system -o deckhouse-<id>
   ```

6. Убедитесь, что новый под Deckhouse запустился корректно:
   ```shell
   kubectl get pods -n d8-system
   ```
7. Теперь можно удалить другие не поды `deckhouse` в некорректном статусе

8. Далее повторите [штатную процедуру](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/admin/configuration/registry/third-party.html) смены registry, заменив MY-PASSWORD на ваш токен, и указав нужный адрес Registry и редакцию вместо `example`:
   ```shell
   kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user licence-token --password MY-PASSWORD registry-example.deckhouse.ru/deckhouse/example
   ```
