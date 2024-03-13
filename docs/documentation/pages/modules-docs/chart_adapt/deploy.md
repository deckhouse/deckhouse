---
title: "Сделайте форк или скопируйте шаблон репозитория с модулем"
permalink: en/modules-docs/chart-adapt/deploy/
---

## Подключаем модуль

1. Заходим в существующий кластер и подключаем репозиторий с модулями (тот самый, который мы указывали в переменных выше для CI/CD) при помощи создания объекта в Deckhouse.

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

   После успешного создания и синхронизации ModuleSource, мы можем посмотреть какие модули доступны для установки:

   ```sh
   kubectl  get ms hello-world -o jsonpath='{.status.modules[*].name}'
   ```

2. Для установки и последующего обновления определяем ModuleUpdatePolicy для нашего ModuleSource (обратить внимание, что канал обновления теперь указывается в политики обновления, поле .spec.releaseChannel), например:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleUpdatePolicy
   metadata:
     name: hello-world
   spec:
     moduleReleaseSelector:
       labelSelector: # селектор на основе меток, **ВАЖНО** не допускать ситуации когда несколько политик матчат один модуль (см.ниже)
         matchLabels: # в данный момент гарантированно наличие именно меток module и source, по ним и следует описать селектор
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

   Важные моменты:
   * Если какой-либо модуль попадает под labelSelector нескольких политик обновления - новые релизы для данного модуля не будут созданы до тех пор пока двусмысленность в применяемых политиках не будет устранена. В ModuleSource будет указана ошибка с описанием какой модуль подпадает под действие нескольких политик и каких.
   * Наличие политики обновления - обязательное условие для создания нового релиза модуля так как через политику обновления задается релизный канал обновлений, а так же режим обновления (полностью автоматический, автоматический по расписнию или ручной).

3. Проверяем ModuleSource (в статусе не должно быть ошибок и должны быть перечислены доступные модули):

   ```sh
   kubectl get ms hello-world -o yaml
   ```

4. Проверяем, что создались новые ModuleReleases для модулей, подпадающих под ModuleUpdatePolicy, и они в статусе Pending (если режим обновления Manual или Auto с указанием окна обновления, за пределами текущего дня недели/временного интервала) или Deployed (если режим обновления Auto без указания окна обновления или с окном обновления, подходящим под текущий день недели/временной интервал):

   ```sh
   kubectl get mr
   ```

   Если в политике обновления выставлен режим обновления Manual, необходимо в ручную подтвердить необходимость установки новой версии модуля. Для этого необходимо добавить аннотацию на указанный релиз:

   ```sh
   kubectl annotate mr <module_release_name> modules.deckhouse.io/approved="true"
   ```

   Для режима обновления Auto подтверждение не нужно.

5. В случаше успешной установки релизов - ждем перезапуска пода Deckhouse.

   ```sh
   kubectl -n d8-system get pod -l app=deckhouse
   ```

6. Включаем модуль при помощи создания объекта в Deckhouse.

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

7. Спустя какое-то время объекты модуля должны появиться в кластере.

   * Команда для просмотра логов Deckhouse в ожидании объектов модуля:

     ```sh
     kubectl -n d8-system logs deploy/deckhouse -f | jq -rc '.msg'
     ```

   * Команда для полчения объектов:

     ```sh
     kubectl get pods -A | grep hello
     ```

## Переключаем модуль на другой ModuleSource

1. В ситуации когда нужно определенный модуль деплоить из другого ModuleSource необходимо определить под какую политику обновления подпадает модуль:

   ```sh
   kubectl get mr
   ```

   Проверяем колонку UPDATE POLICY для релизов нашего модуля.

2. Удаляем (**ВАЖНО** в случае удаления политики, необходимо сперва убедиться, что у нас нет релизов в состоянии Pending, подпадающих под удаляемую политику) данную политику обновления (или модифицируем так, что бы labelSelector не матчил наш модуль):

   ```sh
   kubectl delete mup <policy_name>
   ```

3. Устанавливаем новый ModuleSource (см. выше ## Подключаем модуль п.1).

4. Создаем новую ModuleUpdatePolicy с указанием правильных меток (source) для нового ModuleSource (см. выше ## Подключаем модуль п.2).

5. Проверяем, что новые ModuleRelease для модуля успешно создаются из нового ModuleSource в соответствии с политикой обновления.

   ```sh
   kubectk get mr
   ```

## Примеры moduleReleaseSelector

1. Применять политику ко всем модулям ModuleSorce `deckhouse`:

   ```yaml
   ...
     moduleReleaseSelector:
       labelSelector:
         matchLabels:
           source: deckhouse
   ...
   ```

2. Применять политику к модулю `deckhouse-admin` независимо от ModuleSource:

   ```yaml
   ...
     moduleReleaseSelector:
       labelSelector:
         matchLabels:
           module: deckhouse-admin
   ...
   ```

3. Применять политику к модулю `deckhouse-admin` из ModuleSource `deckhouse`:

   ```yaml
   ...
     moduleReleaseSelector:
       labelSelector:
         matchLabels:
           module: deckhouse-admin
           source: deckhouse
   
   ...
   ```

4. Применять политику только к модулям `deckhouse-admin` и `secrets-store-integration` в ModuleSource `deckhouse`:

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

5. Применять политику ко всем модулям ModuleSource `deckhouse` кроме `deckhouse-admin`:

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
