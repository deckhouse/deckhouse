---
title: "Переключение редакций"
permalink: ru/admin/configuration/registry/switching-editions.html
description: "Переключение между редакциями Deckhouse Kubernetes Platform. Миграция с Community Edition на Enterprise Edition и управление лицензиями."
lang: ru
---

## Переключение DKP с EE на CSE

{% alert level="warning" %}
Инструкция подразумевает использование публичного адреса container registry: `registry-cse.deckhouse.ru`.

В DKP CSE не поддерживается работа облачных кластеров и некоторых модулей. Подробнее о поддерживаемых модулях можно узнать на странице [сравнения редакций](../../../reference/revision-comparison.html).

Миграция на DKP CSE возможна только с версии DKP EE 1.58, 1.64 или 1.67.

Актуальные версии DKP CSE: 1.58.2 для релиза 1.58, 1.64.1 для релиза 1.64 и 1.67.0 для релиза 1.67. Эти версии потребуется использовать далее для указания переменной `DECKHOUSE_VERSION`.

Переход поддерживается только между одинаковыми минорными версиями, например, с DKP EE 1.64 на DKP CSE 1.64. Переход с версии EE 1.58 на CSE 1.67 потребует промежуточной миграции: сначала на EE 1.64, затем на EE 1.67, и только после этого — на CSE 1.67. Попытки обновить версию на несколько релизов сразу могут привести к неработоспособности кластера.

Deckhouse CSE 1.58 и 1.64 поддерживает Kubernetes версии 1.27, DKP CSE 1.67 поддерживает Kubernetes версий 1.27 и 1.29.

При переключении на DKP CSE возможна временная недоступность компонентов кластера.
{% endalert %}

Для переключения кластера Deckhouse Enterprise Edition на Certified Security Edition выполните следующие действия (все команды выполняются на master-узле кластера от имени пользователя с настроенным контекстом `kubectl` или от имени суперпользователя):

1. Настройте кластер на использование необходимой версии Kubernetes (см. примечание выше про доступные версии Kubernetes). Для этого выполните команду:

   ```shell
   d8 platform edit cluster-configuration
   ```

1. Измените [параметр `kubernetesVersion`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) на необходимое значение, например, `"1.27"` (в кавычках) для Kubernetes 1.27.

1. Сохраните изменения. Узлы кластера начнут последовательно обновляться.

1. Дождитесь окончания обновления. Отслеживать ход обновления можно с помощью команды `d8 k get no`. Обновление можно считать завершенным, когда в выводе команды у каждого узла кластера в колонке `VERSION` появится обновленная версия.

1. Подготовьте переменные с токеном лицензии и создайте [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) для переходной авторизации в `registry-cse.deckhouse.ru`:

   ```shell
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
   d8 k apply -f - <<EOF
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-cse-config.sh
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
       bb-sync-file /etc/containerd/conf.d/cse-registry.toml - containerd-config-file-changed << "EOF_TOML"
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           [plugins."io.containerd.grpc.v1.cri".registry]
             [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
           [plugins."io.containerd.grpc.v1.cri".registry.mirrors."registry-cse.deckhouse.ru"]
             endpoint = ["https://registry-cse.deckhouse.ru"]
           [plugins."io.containerd.grpc.v1.cri".registry.configs]
             [plugins."io.containerd.grpc.v1.cri".registry.configs."registry-cse.deckhouse.ru".auth]
               auth = "$AUTH_STRING"
       EOF_TOML
   EOF
   ```

   Дождитесь завершения синхронизации и появления файла `/etc/containerd/conf.d/cse-registry.toml` на узлах.

   Статус синхронизации можно отследить по значению `UPTODATE` (отображаемое число узлов в этом статусе должно совпадать с общим числом узлов (`NODES`) в группе):

   ```shell
   d8 k get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Пример вывода:

   ```console
   NAME     NODES   READY   UPTODATE
   master   1       1       1
   worker   2       2       2
   ```

   В журнале systemd-сервиса bashible должно появиться сообщение `Configuration is in sync, nothing to do` в результате выполнения следующей команды:

   ```shell
   journalctl -u bashible -n 5
   ```

   Пример вывода:

   ```console
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 bashible.sh[53407]: Successful annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Выполните следующие команды для запуска временного пода DKP CSE для получения актуальных дайджестов и списка модулей:

   ```shell
   DECKHOUSE_VERSION=v<ВЕРСИЯ_DECKHOUSE_CSE>
   # Например, DECKHOUSE_VERSION=v1.58.2
   d8 k run cse-image --image=registry-cse.deckhouse.ru/deckhouse/cse/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

   Как только под перейдёт в статус `Running`, выполните следующие команды:

   ```shell
   CSE_SANDBOX_IMAGE=$(d8 k exec cse-image -- cat deckhouse/candi/images_digests.json | grep pause | grep -oE 'sha256:\w*')
   CSE_K8S_API_PROXY=$(d8 k exec cse-image -- cat deckhouse/candi/images_digests.json | grep kubernetesApiProxy | grep -oE 'sha256:\w*')
   CSE_MODULES=$(d8 k exec cse-image -- ls -l deckhouse/modules/ | awk {'print $9'} |grep -oP "\d.*-\w*" | cut -c5-)
   USED_MODULES=$(d8 k get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
   MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $CSE_MODULES | tr ' ' '\n'))
   CSE_DECKHOUSE_KUBE_RBAC_PROXY=$(d8 k exec cse-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.kubeRbacProxy")
   ```

   Дополнительная команда, которая необходима только при переключении на DKP CSE версии 1.64:

   ```shell
   CSE_DECKHOUSE_INIT_CONTAINER=$(d8 k exec cse-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.init")
   ```

1. Убедитесь, что используемые в кластере модули поддерживаются в DKP CSE.
   Например, в Deckhouse CSE 1.58 и 1.64 отсутствует [модуль `cert-manager`](/modules/cert-manager/). Поэтому, перед отключением модуля `cert-manager` необходимо перевести режим работы HTTPS некоторых компонентов (например [`user-authn`](/modules/user-authn/configuration.html#parameters-https-mode) или [`prometheus`](/modules/prometheus/configuration.html#parameters-https-mode)) на альтернативные варианты работы, либо изменить [глобальный параметр](../../../../reference/api/global.html#parameters-modules-https-mode) отвечающий за режим работы HTTPS в кластере.  

   Отобразить список модулей, которые не поддерживаются в DKP CSE и будут отключены, можно следующей командой:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   Проверьте список и убедитесь, что функциональность указанных модулей не задействована вами в кластере, и вы готовы к их отключению.

   Отключите неподдерживаемые в DKP CSE модули:

   ```shell
   echo $MODULES_WILL_DISABLE | 
     tr ' ' '\n' | awk {'print "d8 platform module disable",$1'} | bash
   ```

   В DKP CSE не поддерживается компонент earlyOOM. Отключите его с помощью [настройки](/modules/node-manager/configuration.html#parameters-earlyoomenabled).

   Дождитесь перехода пода DKP в статус `Ready` и выполнения всех задач в очереди.

   ```shell
   d8 platform queue list
   ```

   Проверьте, что отключенные модули перешли в состояние `Disabled`.

   ```shell
   d8 k get modules
   ```

1. Создайте [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration):

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: cse-set-sha-images.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 50
     content: |
        _on_containerd_config_changed() {
          bb-flag-set containerd-need-restart
        }
        bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'

        bb-sync-file /etc/containerd/conf.d/cse-sandbox.toml - containerd-config-file-changed << "EOF_TOML"
        [plugins]
          [plugins."io.containerd.grpc.v1.cri"]
            sandbox_image = "registry-cse.deckhouse.ru/deckhouse/cse@$CSE_SANDBOX_IMAGE"
        EOF_TOML

        sed -i 's|image: .*|image: registry-cse.deckhouse.ru/deckhouse/cse@$CSE_K8S_API_PROXY|' /var/lib/bashible/bundle_steps/051_pull_and_configure_kubernetes_api_proxy.sh
        sed -i 's|crictl pull .*|crictl pull registry-cse.deckhouse.ru/deckhouse/cse@$CSE_K8S_API_PROXY|' /var/lib/bashible/bundle_steps/051_pull_and_configure_kubernetes_api_proxy.sh
   EOF
   ```

   Дождитесь завершения синхронизации `bashible` на всех узлах.

   Состояние синхронизации можно отследить по значению `UPTODATE` статуса (отображаемое число узлов в этом статусе должно совпадать с общим числом узлов (`NODES`) в группе):

   ```shell
   d8 k get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   В журнале systemd-сервиса `bashible` на узлах должно появиться сообщение `Configuration is in sync, nothing to do` в результате выполнения следующей команды:

   ```shell
   journalctl -u bashible -n 5
   ```

   Пример вывода:

   ```console
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 bashible.sh[53407]: Successful annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Актуализируйте секрет доступа к registry DKP CSE, выполнив следующую команду:

   ```shell
   d8 k -n d8-system create secret generic deckhouse-registry \
     --from-literal=".dockerconfigjson"="{\"auths\": { \"registry-cse.deckhouse.ru\": { \"username\": \"license-token\", \"password\": \"$LICENSE_TOKEN\", \"auth\": \"$AUTH_STRING\" }}}" \
     --from-literal="address"=registry-cse.deckhouse.ru \
     --from-literal="path"=/deckhouse/cse \
     --from-literal="scheme"=https \
     --type=kubernetes.io/dockerconfigjson \
     --dry-run='client' \
     -o yaml | kubectl -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- kubectl replace -f -
   ```

1. Измените образ DKP на образ DKP CSE:

   Команда для DKP CSE версии 1.58:

   ```shell
   d8 k -n d8-system set image deployment/deckhouse kube-rbac-proxy=registry-cse.deckhouse.ru/deckhouse/cse@$CSE_DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry-cse.deckhouse.ru/deckhouse/cse:$DECKHOUSE_VERSION
   ```

   Команда для DKP CSE версии 1.64 и 1.67:

   ```shell
   d8 k -n d8-system set image deployment/deckhouse init-downloaded-modules=registry-cse.deckhouse.ru/deckhouse/cse@$CSE_DECKHOUSE_INIT_CONTAINER kube-rbac-proxy=registry-cse.deckhouse.ru/deckhouse/cse@$CSE_DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry-cse.deckhouse.ru/deckhouse/cse:$DECKHOUSE_VERSION
   ```

1. Дождитесь перехода пода DKP в статус `Ready` и выполнения всех задач в очереди. Если в процессе возникает ошибка `ImagePullBackOff`, подождите автоматического перезапуска пода.

   Посмотреть статус пода DKP:

   ```shell
   d8 k -n d8-system get po -l app=deckhouse
   ```

   Проверить состояние очереди DKP:

   ```shell
   d8 platform queue list
   ```

1. Проверьте, не осталось ли в кластере подов с адресом registry для DKP EE:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
     | select(.image | contains("deckhouse.ru/deckhouse/ee"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

   Если в выводе присутствуют поды [модуля `chrony`](/modules/chrony/), заново включите данный модуль (в DKP CSE этот модуль по умолчанию выключен):

   ```shell
   d8 platform module enable chrony
   ```

1. Очистите временные файлы, ресурс NodeGroupConfiguration и переменные:

   ```shell
   rm /tmp/cse-deckhouse-registry.yaml
   d8 k delete ngc containerd-cse-config.sh cse-set-sha-images.sh
   d8 k delete pod cse-image
   ```

   ```yaml
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
       if [ -f /etc/containerd/conf.d/cse-registry.toml ]; then
         rm -f /etc/containerd/conf.d/cse-registry.toml
       fi
       if [ -f /etc/containerd/conf.d/cse-sandbox.toml ]; then
         rm -f /etc/containerd/conf.d/cse-sandbox.toml
       fi
   EOF
   ```

   После синхронизации (статус синхронизации на узлах можно отследить по значению `UPTODATE` у NodeGroup) удалите созданный ресурс NodeGroupConfiguration:

   ```shell
   d8 k delete ngc del-temp-config.sh
   ```

## Переключение DKP на CE/BE/SE/SE+/EE

{% alert level="warning" %}
При использовании [модуля `registry`](/modules/registry/) переключение между редакциями выполняется только в режиме `Unmanaged`.  
Чтобы перейти в режим `Unmanaged`, [воспользуйтесь инструкцией](/modules/registry/examples.html).
{% endalert %}

{% alert level="warning" %}
- Работоспособность инструкции подтверждена только для версий Deckhouse от `v1.70`. Если ваша версия младше, используйте соответствующую ей документацию.
- Для коммерческих изданий требуется действующий лицензионный ключ с поддержкой нужного издания. При необходимости можно [запросить временный ключ](/products/enterprise_edition.html).
- Инструкция подразумевает использование публичного адреса container registry: `registry.deckhouse.ru`. В случае использования другого адреса container registry измените команды или воспользуйтесь [инструкцией по переключению Deckhouse на использование стороннего registry](./third-party.html).
- В редакциях Deckhouse CE/BE/SE/SE+ не поддерживается работа облачных провайдеров Dynamix, Openstack, VCD, vSphere (vSphere поддерживается в редакции SE+) и ряда модулей.
- Все команды выполняются на master-узле существующего кластера под пользователем `root`.
{% endalert %}

Ниже описаны шаги для переключения кластера с любой редакцию на одну из поддерживаемых: Community Edition, Basic Edition, Standard Edition, Standard Edition+, Enterprise Edition.

1. Подготовьте переменные с токеном лицензии и названием новой редакции:

   > Заполнять переменные `NEW_EDITION` и `AUTH_STRING` при переключении на редакцию Deckhouse CE не требуется.
   > Значение переменной `NEW_EDITION` должно быть равно желаемой редакции Deckhouse, например для переключения на редакцию:
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

1. Проверьте, чтобы очередь DKP была пустой и без ошибок:

   ```shell
   d8 system queue list
   ```

1. Создайте ресурс [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) для переходной авторизации в `registry.deckhouse.ru`:

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
   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   d8 k run $NEW_EDITION-image --image=registry.deckhouse.ru/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

1. После перехода пода в статус `Running` выполните следующие команды:

   ```shell
   NEW_EDITION_MODULES=$(d8 k exec $NEW_EDITION-image -- ls -l deckhouse/modules/ | grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)
   USED_MODULES=$(d8 k get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
   MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $NEW_EDITION_MODULES | tr ' ' '\n'))
   ```

1. Убедитесь, что используемые в кластере модули поддерживаются в желаемой редакции.

   Посмотреть список модулей, которые не поддерживаются в новой редакции и будут отключены:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   > Проверьте полученный список и убедитесь, что функциональность указанных модулей не используется вами в кластере и вы готовы их отключить.

   Отключите неподдерживаемые новой редакцией модули:

   ```shell
   echo $MODULES_WILL_DISABLE | tr ' ' '\n' | awk {'print "d8 platform module disable",$1'} | bash
   ```

   Дождитесь, пока под Deckhouse перейдёт в состояние `Ready` и убедитесь в выполнении всех задач в очереди.

1. Выполните команду `deckhouse-controller helper change-registry` из пода Deckhouse с параметрами новой редакции:

   Для переключения на BE/SE/SE+/EE издания:

   ```shell
   DOCKER_CONFIG_JSON=$(echo -n "{\"auths\": {\"registry.deckhouse.io\": {\"username\": \"license-token\", \"password\": \"${LICENSE_TOKEN}\", \"auth\": \"${AUTH_STRING}\"}}}" | base64 -w 0)
   d8 k --as system:sudouser -n d8-cloud-instance-manager patch secret deckhouse-registry --type merge --patch="{\"data\":{\".dockerconfigjson\":\"$DOCKER_CONFIG_JSON\"}}"  
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user=license-token --password=$LICENSE_TOKEN --new-deckhouse-tag=$DECKHOUSE_VERSION registry.deckhouse.io/deckhouse/$NEW_EDITION
   ```

   Для переключения на CE издание:

   ```shell
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --new-deckhouse-tag=$DECKHOUSE_VERSION registry.deckhouse.ru/deckhouse/ce
   ```

1. Проверьте, не осталось ли в кластере подов со старым адресом registry, где `<YOUR-PREVIOUS-EDITION>` — название вашей прошлой редакции:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[] | select(.image | contains("deckhouse.ru/deckhouse/<YOUR-PREVIOUS-EDITION>"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

1. Удалите временные файлы, ресурс NodeGroupConfiguration и переменные:

   > При переходе на редакцию Deckhouse CE пропустите этот шаг.

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
