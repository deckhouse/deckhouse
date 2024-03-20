---
title: "Разверните модуль в кластере"
permalink: ru/modules-docs/chart-adapt/deploy/
lang: ru
---

## Подключение модуля

1. Зайдите в существующий кластер и подключите репозиторий с модулями для CI/CD (указанный выше) при помощи создания объекта в Deckhouse.

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleSource
   metadata:
     name: hello-world
   spec:
     releaseChannel: alpha #deprecated field
     registry:
       # Пример: dev-registry.deckhouse.io/deckhouse/modules-source
       repo: <ваш регистри></путь/до/репозитория/с/модулями>
       # Строка в формате [dockerconfigjson](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/#registry-secret-existing-credentials)
       #
       # Пример: 
       # base64 -w0 <<EOF
       # {
       #   "auths": {
       #     "dev-registry.deckhouse.io": {
       #       "auth": "$(echo -n 'username:password' | base64 -w0)"
       #     }
       #   }
       # }
       # EOF
       dockerCfg: <base64 encoded credentials>
   ```

   После успешного создания и синхронизации *ModuleSource*, посмотрите какие модули доступны для установки:

   ```sh
   kubectl  get ms hello-world -o jsonpath='{.status.modules[*].name}'
   ```

1. Для установки и последующего обновления, определите *ModuleUpdatePolicy* для *ModuleSource* (обратите внимание, что канал обновления теперь указывается в политике обновления в параметре `.spec.releaseChannel`), например:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleUpdatePolicy
   metadata:
     name: hello-world
   spec:
     moduleReleaseSelector:
       labelSelector: # селектор на основе лейблов. Важно избегать ситуации, когда один модуль соответствует нескольким политикам обновления (см. ниже).
         matchLabels: # гарантированно наличие лейблов module и source, по ним необходимо описать селектор
           module: hello-world-server
           source: hello-world
     releaseChannel: Alpha
     update:
       mode: Auto # <Auto|Manual>
       windows:
       - days:
         - "Mon"
         - "Tue"
         - "Wed"
         from: "13:30" # время UTC
         to: "14:00"
   ```

   Важно:
* Если какой-либо модуль попадает под лейбл *labelSelector* нескольких политик обновления, новые релизы для этого модуля не будут создаваться до тех пор, пока не будет устранена неоднозначность в применяемых политиках. В *ModuleSource* указывается ошибка с пояснением, какой модуль затрагивается несколькими политиками и какими именно.
* Наличие политики обновления является обязательным условием для создания нового релиза модуля, так как через эту политику определяется канал обновления и режим обновления (полностью автоматический, обновляющийся по расписанию или ручной).

1. Проверьте *ModuleSource* (в статусе не должно содержаться ошибок и должны быть перечислены доступные модули):

   ```sh
   kubectl get ms hello-world -o yaml
   ```

1. Убедитесь, что были созданы новые ресурсы *ModuleRelease* для модулей, подпадающих под *ModuleUpdatePolicy*, и что они имеют статус Pending (если режим обновления ручной или автоматический с указанием окна обновления за пределами текущей даты/временного интервала) или Deployed (при условии, что обновление является автоматическим без указания окна обновления, или окно обновления совпадает с текущей датой/временным интервалом):

   ```sh
   kubectl get mr
   ```

   Если в политике обновления выставлен ручной режим обновления, необходимо в ручную подтвердить установку новой версии модуля. Для этого добавьте аннотацию на указанный релиз:

   ```sh
   kubectl annotate mr <module_release_name> modules.deckhouse.io/approved="true"
   ```

   Для автоматического режима обновления подтверждение не нужно.

1. В случае успешной установки релизов, дождитесь перезапуска пода Deckhouse Kubernetes Platform.

   ```sh
   kubectl -n d8-system get pod -l app=deckhouse
   ```

1. Включите модуль при помощи создания объекта в Deckhouse Kubernetes Platform.

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: hello-world
   spec:
     enabled: true
     settings: {}
     version: 1
   ```

Через некоторое время объекты модуля появятся в кластере.

* Команда для просмотра логов Deckhouse Kubernetes Platform в ожидании объектов модуля:

     ```sh
     kubectl -n d8-system logs deploy/deckhouse -f | jq -rc '.msg'
     ```

* Команда для получения объектов:

     ```sh
     kubectl get pods -A | grep hello
     ```

## Переключение модуля на другой ModuleSource

1. Если необходимо развернуть определенный модуль из другого *ModuleSource*, определите, под какую политику обновлений подпадает этот модуль:

   ```sh
   kubectl get mr
   ```

   Проверьте `UPDATE POLICY` для релизов модуля.

2. Прежде чем удалить эту политику обновления, убедитесь, что нет ожидающих развертывания (в состоянии Pending) релизов, которые подпадают под удаляемую или изменяемую политику (или *labelSelector*, используемый политикой, больше не соответствует вашему модулю):

   ```sh
   kubectl delete mup <policy_name>
   ```

3. Установите новый *ModuleSource* (см. раздел ## Подключение модуля п.1).

4. Создайте новую *ModuleUpdatePolicy* с указанием правильных меток (source) для нового *ModuleSource* (см. раздел ## Подключение модуля п.2).

5. Проверьте, что новые *ModuleRelease* для модуля создаются из нового *ModuleSource* в соответствии с политикой обновления.

   ```sh
   kubectk get mr
   ```

## Примеры moduleReleaseSelector

1. Примените политику ко всем модулям *ModuleSorce* `deckhouse`:

   ```yaml
   ...
     moduleReleaseSelector:
       labelSelector:
         matchLabels:
           source: deckhouse
   ...
   ```

2. Примените политику к модулю `deckhouse-admin` независимо от *ModuleSource*:

   ```yaml
   ...
     moduleReleaseSelector:
       labelSelector:
         matchLabels:
           module: deckhouse-admin
   ...
   ```

3. Примените политику к модулю `deckhouse-admin` из *ModuleSource* `deckhouse`:

   ```yaml
   ...
     moduleReleaseSelector:
       labelSelector:
         matchLabels:
           module: deckhouse-admin
           source: deckhouse
   
   ...
   ```

4. Примените политику только к модулям `deckhouse-admin` и `secrets-store-integration` в *ModuleSource* `deckhouse`:

   ```yaml
   ...
     moduleReleaseSelector:
       labelSelector:
         matchExpressions:
         - key: module
           operator: In
           values:
           - deckhouse-admin
           - secrets-store-integration
         matchLabels:
           source: deckhouse
   ...
   ```

5. Примените политику ко всем модулям *ModuleSource* `deckhouse`, кроме `deckhouse-admin`:

   ```yaml
   ...
     moduleReleaseSelector:
       labelSelector:
         matchExpressions:
         - key: module
           operator: NotIn
           values:
           - deckhouse-admin
         matchLabels:
           source: deckhouse
   ...
   ```
