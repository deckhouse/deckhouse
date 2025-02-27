---
title: "Доступ к API"
permalink: ru/admin/configuration/access/api.html
lang: ru
---

TODO
- [через балансер](Доступ к Kubernetes API через балансировщик трафика) - настройка подключения через публикацию API через ingress
- через сервис - прямой доступ к апи, в обход ингресс-контроллера (уточнить технические нюансы). Добавить про плюсы и минусы.
- при помощи basic-auth

## Доступ к Kubernetes API через балансировщик трафика

DKP позволяет организовать доступ к Kubernetes API через балансировщик трафика (Ingress-контроллер). В этом случае, для доступа к Kubernetes API, например, через `kubectl`, пользователю будет необходимо с помощью веб-интерфейса сгенерировать файл настроек для `kubectl`. Попасть в веб-интерфейс пользователь может только пройдя аутентификацию.

Чтобы включить доступ к Kubernetes API через балансировщик трафика, установите параметр [publishAPI](configuration.html#parameters-publishapi) в `true` в настройках модуля `user-authn`. Это можно сделать как через веб-интерфейс администратора, так и через CLI.

Включение через CLI (требуется `kubectl` настроенный на работу с кластером):

- Включите модуль `user-authn`, если он не включен.

  Проверить статус модуля:
  
  ```shell
  kubectl get module user-authn
  ```

  Пример вывода:  

  ```console
  kubectl get module user-authn
  NAME         WEIGHT   SOURCE     PHASE   ENABLED   READY
  user-authn   150      Embedded   Ready   True      True
  ```

  Включить модуль:

  ```shell
  kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module enable user-authn
  ```
  
- Откройте настройки модуля `user-authn` (создайте ресурс moduleConfig `user-authn`, если его нет):

  ```shell
  kubectl edit mc user-authn
  ```

- Установите параметр [publishAPI](configuration.html#parameters-publishapi) в `true` и сохраните изменения.

  Пример настройки модуля `user-authn`:

  ```yaml
  spec:
    enabled: true
    settings:
      publishAPI:
        enabled: true
  version: 2  
  ```

Веб-интерфейс будет доступен через несколько секунд после сохранения изменений. Для веб-интерфейса зарезервировано имя `kubeconfig`, и конечный URL будет зависеть от шаблона DNS-имен, указанного в глобальном параметре `publicDomainTemplate` DKP. Узнать URL веб-интерфейса можно в интерфейсе администратора, в разделе Web Interfaces главной страницы Grafana или с помощью команды:

```shell
kubectl -n d8-user-authn get ingress kubeconfig-generator -o jsonpath='{.spec.rules[*].host}'
```
