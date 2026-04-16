---
title: "Переключение между редакциями DKP (кроме CSE)"
permalink: ru/admin/configuration/update/switching-editions.html
description: "Переключение между редакциями Deckhouse Kubernetes Platform. Миграция с Community Edition на Enterprise Edition и управление лицензиями."
lang: ru
---

Эта инструкция описывает шаги, необходимые для смены редакции Deckhouse Kubernetes Platform в работающем кластере, кроме переключения на DKP CSE (Deckhouse Kubernetes Platform Certified Security Edition). Для переключения на DKP CSE воспользуйтесь [отдельной инструкцией](switching-cse.html).

В DKP реализовано два способа работы с хранилищем образов контейнеров платформы (регистри, хранилище образов):
- с помощью модуля `registry` — рекомендованный способ, при котором конфигурация работы с хранилищем образов задаётся в секции [registry](/modules/deckhouse/configuration.html#parameters-registry) параметров модуля `deckhouse` (ModuleConfig `deckhouse`). Этот способ обеспечивает более плавный процесс перехода и автоматическую проверку наличия необходимых образов.
- без использования модуля `registry` — **устаревший способ**, при котором конфигурация работы с хранилищем образов задаётся при установке кластера [в InitConfiguration](../reference/api/cr.html#initconfiguration-deckhouse-imagesrepo) и параметр [registry.mode](/modules/deckhouse/configuration.html#parameters-registry-mode) модуля `deckhouse` (ModuleConfig `deckhouse`) установлен в `Unmanaged`.

В зависимости от способа работы с хранилищем образов, процесс переключения между редакциями DKP отличается. Также, при переключении на DKP CE не требуется указывать лицензионный ключ и выполнять авторизацию в хранилище образов контейнеров. При переключении на редакции BE/SE/SE+/EE будет необходимо указать действующий лицензионный ключ и выполнить авторизацию в хранилище образов контейнеров.

{% alert level="warning" %}
Инструкция подразумевает использование публичного адреса хранилища образов контейнеров: `registry.deckhouse.ru`. В случае использования другого адреса хранилища образов измените команды или воспользуйтесь [инструкцией по переключению Deckhouse на использование стороннего хранилища образов контейнеров](./registry/third-party.html).

Все команды выполняются на master-узле существующего кластера под пользователем `root`.
{% endalert %}


## Подготовка перед переключением

### Проверьте очередь Deckhouse

Убедитесь, что очередь пуста:

```bash
d8 system queue list
```

Пример вывода:

```console
Summary:
- 'main' queue: empty.
- 88 other queues (0 active, 88 empty): 0 tasks.
- no tasks to handle.
```

### Подготовьте данные для авторизации в хранилище образов контейнеров

При переключении на DKP CE в публичном хранилище образов контейнеров (registry.deckhouse.ru) данные для авторизации не требуются — пропустите этот шаг.

Для переключения на DKP коммерческих редакций BE/SE/SE+/EE подготовьте лицензионный ключ, действующий для редакции, на которую вы планируете переключиться.

### Определите текущую редакцию DKP

Узнать текущую редакцию DKP, используемую в кластере, можно на главной странице веб-интерфейса DKP, либо выполнив следующую команду:

```bash
d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller global values -o yaml | yq '.deckhouseEdition'
```

### Убедитесь в возможности переключения на желаемую редакцию DKP


1. проверить, какие модули доступны в новой редакции;
1. отключить модули, которые новая редакция не поддерживает;



### Переключение с помощью модуля registry

1. Убедитесь, что кластер был переключен на использование модуля [`registry`](/modules/registry/faq.html#как-мигрировать-на-модуль-registry). Если модуль не используется, перейдите [к инструкции](#переключение-без-использования-модуля-registry).

1. Подготовьте переменные с лицензионным ключом и названием новой редакции:

   > Заполнять переменную `LICENSE_TOKEN` при переключении на редакцию CE не требуется.
   > Значение переменной `NEW_EDITION` должно быть равно желаемой редакции DKP, например для переключения на редакцию:
   > - CE, переменная должна быть `ce`;
   > - BE, переменная должна быть `be`;
   > - SE, переменная должна быть `se`;
   > - SE+, переменная должна быть `se-plus`;
   > - EE, переменная должна быть `ee`.

   ```shell
   NEW_EDITION=<PUT_YOUR_EDITION_HERE>
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   ```

1. Проверьте, чтобы очередь Deckhouse была пустой и без ошибок:

   ```shell
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

   Пример вывода (очереди пусты):

   ```console
   Summary:
   - 'main' queue: empty.
   - 88 other queues (0 active, 88 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Запустите временный под Deckhouse новой редакции, чтобы получить актуальные дайджесты и список модулей:

   Для CE редакции:

   ```shell
   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
   d8 k run $NEW_EDITION-image --image=registry.deckhouse.ru/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

   Для других редакций:

   ```shell
   d8 k create secret docker-registry $NEW_EDITION-image-pull-secret \
    --docker-server=registry.deckhouse.ru \
    --docker-username=license-token \
    --docker-password=${LICENSE_TOKEN}

   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
   d8 k run $NEW_EDITION-image \
    --image=registry.deckhouse.ru/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION \
    --overrides="{\"spec\": {\"imagePullSecrets\":[{\"name\": \"$NEW_EDITION-image-pull-secret\"}]}}" \
    --command sleep -- infinity
   ```

   Как только под перейдёт в статус `Running`, выполните следующие команды:

   ```shell
   NEW_EDITION_MODULES=$(d8 k exec $NEW_EDITION-image -- ls -l deckhouse/modules/ | grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)
   USED_MODULES=$(d8 k get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
   MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $NEW_EDITION_MODULES | tr ' ' '\n'))
   ```

1. Убедитесь, что используемые в кластере модули поддерживаются в желаемой редакции.

   Посмотреть список модулей, которые не поддерживаются в новой редакции и будут отключены, можно с помощью команды:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   > Проверьте полученный список и убедитесь, что функциональность указанных модулей не используется вами в кластере и вы готовы их отключить.

   Отключите неподдерживаемые новой редакцией модули:

   ```shell
   echo $MODULES_WILL_DISABLE | tr ' ' '\n' | awk {'print "d8 platform module disable",$1'} | bash
   ```

   Дождитесь, пока под Deckhouse перейдёт в состояние `Ready` и убедитесь в выполнении всех задач в очереди:

   ```shell
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

   Пример вывода (очереди пусты):

   ```console
   Summary:
   - 'main' queue: empty.
   - 88 other queues (0 active, 88 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Удалите созданный секрет и под:

   ```shell
   d8 k delete pod/$NEW_EDITION-image
   d8 k delete secret/$NEW_EDITION-image-pull-secret
   ```

1. Выполните переключение на новую редакцию. Для этого укажите следующие параметры в ModuleConfig `deckhouse` (для подробной настройки ознакомьтесь с конфигурацией модуля [`deckhouse`](/modules/deckhouse/)):

   ```yaml
   ---
   # Пример для Direct режима
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Direct
         direct:
           # Relax mode используется для проверки наличия текущей версии Deckhouse в указанном хранилище образов
           # Для переключения между редакциями необходимо использовать данный режим проверки хранилища образов
           checkMode: Relax
           # Укажите свой параметр <NEW_EDITION>
           imagesRepo: registry.deckhouse.ru/deckhouse/<NEW_EDITION>
           scheme: HTTPS
           # Укажите свой параметр <LICENSE_TOKEN>
           # Если переключение выполняется на CE редакцию, удалите данный параметр
           license: <LICENSE_TOKEN>
   ---
   # Пример для Unmanaged режима
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
         unmanaged:
           # Relax mode используется для проверки наличия текущей версии Deckhouse в указанном хранилище образов
           # Для переключения между редакциями необходимо использовать данный режим проверки
           checkMode: Relax
           # Укажите свой параметр <NEW_EDITION>
           imagesRepo: registry.deckhouse.ru/deckhouse/<NEW_EDITION>
           scheme: HTTPS
           # Укажите свой параметр <LICENSE_TOKEN>
           # Если переключение выполняется на CE редакцию, удалите данный параметр
           license: <LICENSE_TOKEN>
   ```

1. Дождитесь переключения хранилища образов контейнеров. Для проверки выполнения переключения воспользуйтесь [инструкцией](/modules/registry/faq.html#как-посмотреть-статус-переключения-режима-registry).

   Пример вывода:

   ```yaml
   conditions:
     - lastTransitionTime: "..."
       message: |-
         Mode: Relax
         registry.deckhouse.ru: all 1 items are checked
       reason: Ready
       status: "True"
       type: RegistryContainsRequiredImages
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   ```

1. После переключения, удалите из ModuleConfig `deckhouse` параметр `checkMode: Relax`, чтобы активировать выполнение проверки по умолчанию. Удаление запустит проверку наличия критически важных компонентов в хранилище образов контейнеров.

1. Дождитесь выполнения проверки. Статус переключения режима хранилища образов можно получить, воспользовавшись [инструкцией](/modules/registry/faq.html#как-посмотреть-статус-переключения-режима-registry).

   Пример вывода:

   ```yaml
   conditions:
     - lastTransitionTime: "..."
       message: |-
         Mode: Default
         registry.deckhouse.ru: all 155 items are checked
       reason: Ready
       status: "True"
       type: RegistryContainsRequiredImages
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   ```

1. Проверьте, нет ли в неймспейсах `d8-*` подов в состоянии ошибки, которые не могут загрузить образы. Это необходимо сделать вручную, так как в настоящий момент модули Deckhouse не переинициализируются автоматически после изменений, описанных выше.

   Получите список подов:

   ```shell
   d8 k get po -A
   ```

   Получите детальную информацию о проблемных подах:

   ```shell
   d8 k describe po <pod_name> <namespace>
   ```

   Повторно загрузите соответствующие проблемным подам модули, выполнив на всех master-узлах команду:

   ```shell
   rm -rf /var/lib/deckhouse/downloaded/<module-name>/
   ```

   Для получения `<module-name>` выполните команду:

   ```shell
   d8 k get modules
   ```

   После удаления данных нужных модулей перезапустите Deckhouse:

   ```shell
   d8 k rollout restart deploy -n d8-system deckhouse
   ```

1. Проверьте, не осталось ли в кластере подов со старым адресом хранилища образов контейнеров, где `<YOUR-PREVIOUS-EDITION>` — название вашей прошлой редакции:

   Для Unmanaged-режима:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[] | select(.image | contains("deckhouse.ru/deckhouse/<YOUR-PREVIOUS-EDITION>"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

   Для других режимов, использующих фиксированный адрес (данная проверка не учитывает внешние модули):

   ```shell
   # Получаем список актуальных digest'ов из файла images_digests.json внутри Deckhouse
   IMAGES_DIGESTS=$(d8 k -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- cat /deckhouse/modules/images_digests.json | jq -r '.[][]' | sort -u)

   # Проверяем, есть ли поды, использующие образы Deckhouse по адресу `registry.d8-system.svc:5001/system/deckhouse`
   # с digest'ом, отсутствующим в списке актуальных digest'ов из IMAGES_DIGESTS
   d8 k get pods -A -o json |
   jq -r --argjson digests "$(printf '%s\n' $IMAGES_DIGESTS | jq -R . | jq -s .)" '
     .items[]
     | {name: .metadata.name, namespace: .metadata.namespace, containers: .spec.containers}
     | select(.containers != null)
     | select(
         .containers[]
         | select(.image | test("registry.d8-system.svc:5001/system/deckhouse") and test("@sha256:"))
         | .image as $img
         | ($img | split("@") | last) as $digest
         | ($digest | IN($digests[]) | not)
       )
     | .namespace + "\t" + .name
   ' | sort -u
   ```

### Переключение без использования модуля registry

1. Если модуль `registry` включен, отключите его с [помощью инструкции](/modules/registry/faq.html#как-мигрировать-обратно-с-модуля-registry).

1. Подготовьте переменные с лицензионным ключом и названием новой редакции:

   > Заполнять переменные `NEW_EDITION` и `AUTH_STRING` при переключении на редакцию CE не требуется.
   > Значение переменной `NEW_EDITION` должно быть равно желаемой редакции DKP, например для переключения на редакцию:
   > - CE, переменная должна быть `ce`;
   > - BE, переменная должна быть `be`;
   > - SE, переменная должна быть `se`;
   > - SE+, переменная должна быть `se-plus`;
   > - EE, переменная должна быть `ee`.

   ```shell
   NEW_EDITION=<PUT_YOUR_EDITION_HERE>
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
   ```

1. Проверьте, чтобы очередь Deckhouse была пустой и без ошибок:

   ```shell
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

   Пример вывода (очереди пусты):

   ```console
   Summary:
   - 'main' queue: empty.
   - 88 other queues (0 active, 88 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Создайте ресурс `NodeGroupConfiguration` для переходной авторизации в `registry.deckhouse.ru`:

   > Перед созданием ресурса ознакомьтесь с разделом [Как добавить конфигурацию для дополнительного хранилища образов контейнеров](/modules/node-manager/faq.html#как-добавить-конфигурацию-для-дополнительного-registry).
   >
   > При переходе на редакцию Deckhouse CE пропустите этот шаг.

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-$NEW_EDITION-config.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 30
     content: |
       _on_containerd_config_changed() {
         bb-flag-set containerd-need-restart
       }
       bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'
       mkdir -p /etc/containerd/conf.d
       bb-sync-file /etc/containerd/conf.d/$NEW_EDITION-registry.toml - containerd-config-file-changed << "EOF_TOML"
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           [plugins."io.containerd.grpc.v1.cri".registry.configs]
             [plugins."io.containerd.grpc.v1.cri".registry.configs."registry.deckhouse.ru".auth]
               auth = "$AUTH_STRING"
       EOF_TOML
   EOF
   ```

   Дождитесь появления файла `/etc/containerd/conf.d/$NEW_EDITION-registry.toml` на узлах и завершения синхронизации bashible. Чтобы отследить статус синхронизации, проверьте значение `UPTODATE` (число узлов в этом статусе должно совпадать с общим числом узлов (`NODES`) в группе):

   ```shell
   d8 k get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Пример вывода:
  
   ```console
   NAME     NODES   READY   UPTODATE
   master   1       1       1
   worker   2       2       2
   ```

   Также в журнале systemd-сервиса bashible должно появиться сообщение `Configuration is in sync, nothing to do` в результате выполнения следующей команды:

   ```shell
   journalctl -u bashible -n 5
   ```

   Пример вывода:

   ```console
   Aug 21 11:04:28 master-ee-to-se-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-se-0 bashible.sh[53407]: Annotate node master-ee-to-se-0 with annotation node.deckhouse.io/   configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master ee-to-se-0 bashible.sh[53407]: Successful annotate node master-ee-to-se-0 with annotation node.deckhouse.io/   configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-se-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Запустите временный под Deckhouse новой редакции, чтобы получить актуальные дайджесты и список модулей:

   ```shell
   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
   d8 k run $NEW_EDITION-image --image=registry.deckhouse.ru/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

1. После перехода пода в статус `Running` выполните следующие команды:

   ```shell
   NEW_EDITION_MODULES=$(d8 k exec $NEW_EDITION-image -- ls -l deckhouse/modules/ | grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)
   USED_MODULES=$(d8 k get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
   MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $NEW_EDITION_MODULES | tr ' ' '\n'))
   ```

1. Убедитесь, что используемые в кластере модули поддерживаются в желаемой редакции.

   Посмотреть список модулей, которые не поддерживаются в новой редакции и будут отключены, можно с помощью команды:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   > Проверьте полученный список и убедитесь, что функциональность указанных модулей не используется в кластере и вы готовы их отключить.

   Отключите неподдерживаемые новой редакцией модули:

   ```shell
   echo $MODULES_WILL_DISABLE | tr ' ' '\n' | awk {'print "d8 platform module disable",$1'} | bash
   ```

   Дождитесь, пока под Deckhouse перейдёт в состояние `Ready` и убедитесь в выполнении всех задач в очереди:

   ```shell
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

   Пример вывода (очереди пусты):

   ```console
   Summary:
   - 'main' queue: empty.
   - 88 other queues (0 active, 88 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Выполните команду `deckhouse-controller helper change-registry` из пода Deckhouse с параметрами новой редакции:

   Для переключения на BE/SE/SE+/EE издания:

   ```shell
   DOCKER_CONFIG_JSON=$(echo -n "{\"auths\": {\"registry.deckhouse.ru\": {\"username\": \"license-token\", \"password\": \"${LICENSE_TOKEN}\", \"auth\": \"${AUTH_STRING}\"}}}" | base64 -w 0)
   d8 k --as system:sudouser -n d8-cloud-instance-manager patch secret deckhouse-registry --type merge --patch="{\"data\":{\".dockerconfigjson\":\"$DOCKER_CONFIG_JSON\"}}"  
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user=license-token --password=$LICENSE_TOKEN --new-deckhouse-tag=$DECKHOUSE_VERSION registry.deckhouse.ru/deckhouse/$NEW_EDITION
   ```

   Для переключения на CE издание:

   ```shell
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --new-deckhouse-tag=$DECKHOUSE_VERSION registry.deckhouse.ru/deckhouse/ce
   ```

1. Проверьте, не осталось ли в кластере подов со старым адресом registry, где `<YOUR-PREVIOUS-EDITION>` — название вашей прошлой редакции:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[] | select(.image | contains("deckhouse.ru/deckhouse/<YOUR-PREVIOUS-EDITION>"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

1. Удалите временные файлы, ресурс `NodeGroupConfiguration` и переменные:

   > При переходе на редакцию CE пропустите этот шаг.

   ```shell
   d8 k delete ngc containerd-$NEW_EDITION-config.sh
   d8 k delete pod $NEW_EDITION-image
   d8 k apply -f - <<EOF
       apiVersion: deckhouse.io/v1alpha1
       kind: NodeGroupConfiguration
       metadata:
         name: del-temp-config.sh
       spec:
         nodeGroups:
         - '*'
         bundles:
         - '*'
         weight: 90
         content: |
           if [ -f /etc/containerd/conf.d/$NEW_EDITION-registry.toml ]; then
             rm -f /etc/containerd/conf.d/$NEW_EDITION-registry.toml
           fi
   EOF
   ```

   После завершения синхронизации bashible (статус синхронизации на узлах отображается по значению `UPTODATE` у NodeGroup) удалите созданный ресурс NodeGroupConfiguration:

   ```shell
   d8 k delete ngc del-temp-config.sh
   ```
