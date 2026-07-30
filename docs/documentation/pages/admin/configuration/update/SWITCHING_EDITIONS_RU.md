---
title: "Переключение между редакциями DKP"
permalink: ru/admin/configuration/update/switching-editions.html
description: "Переключение между редакциями Deckhouse Kubernetes Platform."
lang: ru
---

Эта инструкция описывает шаги, необходимые для смены редакции Deckhouse Kubernetes Platform в работающем кластере. Выполняйте их последовательно по разделам.

В зависимости от способа работы с хранилищем образов процесс переключения отличается. Выбираете подходящий для вашего кластера способ и следуйте инструкциям.

При переключении на DKP BE/SE/SE+/EE/CSE необходим действующий лицензионный ключ. При переключении на DKP CE он не требуется.

{% alert level="warning" %}
Инструкция не подходит для переключения **с** DKP CSE на другие редакции, но подходит для переключения **на** DKP CSE с DKP EE.

Инструкция подразумевает использование публичного хранилища образов контейнеров (`registry-cse.deckhouse.ru` для DKP CSE, и `registry.deckhouse.ru` в остальных случаях). При использовании другого адреса хранилища образов измените команды или воспользуйтесь [инструкцией по переключению Deckhouse на использование стороннего хранилища образов контейнеров](../registry/third-party.html).

Выполняйте все команды на master-узле существующего кластера под пользователем `root`.
{% endalert %}

{% capture wait_queue %}

```bash
d8 system queue list
```

{% offtopic title="Пример вывода (очереди пусты)..." %}

```console
Summary:
- 'main' queue: empty.
- 88 other queues (0 active, 88 empty): 0 tasks.
- no tasks to handle.
```

{% endofftopic %}
{% endcapture %}

## Подготовка к переключению

Перед переключением между редакциями выполните следующие действия:

1. Убедитесь, что [очереди DKP пусты](#проверка-очереди).
1. Определите [текущую редакцию и версию DKP](#определение-текущей-редакции-и-версии).
1. Убедитесь в возможности переключения [с текущей редакции на желаемую](#определение-возможности-переключения-на-желаемую-редакцию).

### Проверка очереди

Убедитесь, что очереди DKP пусты, и в них нет выполняющихся задач, которые могут помешать переключению:

{{ wait_queue }}

### Определение текущей редакции и версии

Чтобы быть уверенным в корректности дальнейших действий, определите текущую редакцию DKP, используемую в кластере. Это поможет избежать ошибок при переключении и убедиться в поддержке необходимых модулей и функциональных возможностей в новой редакции.

Узнать используемые в кластере редакцию и версию DKP можно на главной странице веб-интерфейса DKP, либо с помощью CLI-команд:

- получение текущей редакции DKP:

  ```bash
  d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller global values -o yaml | yq '.deckhouseEdition'
  ```

- получение текущей версии DKP:

  ```bash
  d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}'
  ```

### Определение возможности переключения на желаемую редакцию

{% capture take_care_of_the_internal_modules %}
1. Определите список внутренних модулей, которые используются в кластере и не поддерживаются в DKP новой редакции. Для этого выполните следующие шаги:

   <!REMOVE_FOR_CE>

   1. Подготовьте переменную окружения, указав лицензионный ключ для редакции, на которую вы планируете переключиться:

      ```shell
      LICENSE_TOKEN=<ЛИЦЕНЗИОННЫЙ_КЛЮЧ>
      ```

   <!/REMOVE_FOR_CE>

   1. Получите список модулей, которые не поддерживаются в DKP $NEW_EDITION:

      ```shell
      (set -e
      trap 'echo "Ошибка выполнения"' ERR

      echo
      echo "Запуск пода deckhouse новой редакции с командой 'sleep -- infinity'"

      DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
      <!REMOVE_FOR_CE>
      d8 k create secret docker-registry $NEW_EDITION-image-pull-secret --docker-server=registry.deckhouse.ru --docker-username=license-token --docker-password=${LICENSE_TOKEN}
      <!/REMOVE_FOR_CE>
      d8 k run $NEW_EDITION-image --image=registry.deckhouse.ru/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION \
      <!REMOVE_FOR_CE>
      --overrides="{\"spec\": {\"imagePullSecrets\":[{\"name\": \"$NEW_EDITION-image-pull-secret\"}]}}" \
      <!/REMOVE_FOR_CE>
      --command sleep -- infinity
      d8 k wait --for=condition=ready pod/$NEW_EDITION-image --timeout=300s

      echo
      echo "Получение информации по внутренним модулям из текущей и новой редакции"
      echo "Сравнение внутренних модулей текущей и новой редакции"
      NEW_MODULES=$(d8 k exec $NEW_EDITION-image -- ls -l deckhouse/modules/ |   grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)
      USED_MODULES=$(d8 k get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
      MODULES_TO_DISABLE=$(echo "$USED_MODULES" | tr ' ' '\n' | grep -Fxv -f <(echo "$NEW_MODULES" | tr ' ' '\n') 2>&1) || { status=$?; [ $status -eq 1 ] && MODULES_TO_DISABLE="" || exit $status; }

      echo
      echo "Модули, которые не поддерживаются в желаемой редакции (код редакции - $NEW_EDITION, версия - $DECKHOUSE_VERSION):"
      echo MODULES_TO_DISABLE=\"$MODULES_TO_DISABLE\"
      )

      echo
      echo "Удаление пода deckhouse новой редакции"
      d8 k delete pod/$NEW_EDITION-image secret/$NEW_EDITION-image-pull-secret --wait=true --ignore-not-found=true
      ```

   1. Отключите модули из полученного списка, если это допустимо (функциональность модулей не используется, или вы готовы от нее отказаться). Иначе, **прервите процесс переключения.**

      Отключить модули из полученного списка можно в веб-интерфейсе DKP в разделе «Система» → «Управление системой» → «Deckhouse» → «Модули», либо выполнив следующую команду:

      ```shell
      echo $MODULES_TO_DISABLE | tr ' ' '\n' | awk {'print "d8 platform module disable",$1'} | bash
      ```

{% endcapture %}

{% capture take_care_of_the_external_modules %}
1. Определите список внешних модулей, запущенных из `moduleSource/deckhouse`, которые не поддерживаются в DKP новой редакции. Для этого выполните следующие шаги:

   <!REMOVE_FOR_CE>

   1. Подготовьте переменную окружения, указав лицензионный ключ для редакции, на которую вы планируете переключиться:

      ```shell
      LICENSE_TOKEN=<ЛИЦЕНЗИОННЫЙ_КЛЮЧ>
      ```

   <!/REMOVE_FOR_CE>

   1. Получите список внешних модулей запущенных из `moduleSource/deckhouse` с результатом проверки их наличия в DKP новой редакции:

      ```shell
      d8-crane() {
         if command -v d8 &>/dev/null && d8 cr &>/dev/null 2>&1; then
            d8 cr "$@"
         else
            crane "$@"
         fi
      }

      <!REMOVE_FOR_CE>
      d8-crane-login() {
         if command -v d8 &>/dev/null && d8 cr &>/dev/null 2>&1; then
            d8 cr login "$1" -u "license-token" -p "$2"
         else
            crane auth login "$1" -u "license-token" -p "$2"
         fi
      }
      <!/REMOVE_FOR_CE>

      check_external_modules() {
         local repo="$1"

         local modules=$(kubectl get modulerelease -o json | jq -r '.items[] | select(.metadata.ownerReferences[] | select(.kind=="ModuleSource" and .name=="deckhouse")) | "\(.spec.moduleName) \(.spec.version)"')
         [[ -z "$modules" ]] && return 0
         local registry_modules=$(d8-crane ls "$repo" 2>/dev/null)

         echo "Модуль | Наличие модуля | Наличие версии"
         echo "-------|----------------|---------------"

         echo "$modules" | while read module version; do
            grep -qx "$module" <<< "$registry_modules" && list="✅" || list="❌"
            d8-crane manifest "$repo/${module}/release:v${version}" &>/dev/null && release="✅" || release="❌"
            echo "$module ($version) | $list | $release"
         done
      }

      <!REMOVE_FOR_CE>
      d8-crane-login "registry.deckhouse.ru" "$LICENSE_TOKEN"
      <!/REMOVE_FOR_CE>
      check_external_modules "registry.deckhouse.ru/deckhouse/$NEW_EDITION/modules"
      ```

   1. Отключите модули которые не были найден в новой редакции, если это допустимо. Иначе, **прервите процесс переключения.**

{% endcapture %}

{% capture take_care_deckhuse_imagepullbackoff %}
1. Убедитесь, что deckhouse запустился после смены registry:

   1. Для этого, воспользуйтесь командой:

      ```bash
      d8 k -n d8-system get pods -l "app=deckhouse"
      ```

   1. Если Deckhouse находится в ImagePullBackoff с ошибкой скачивания cidecar контейнеров, укажите актуальные образа:

      Получить актуальные дайджесты cidecar образов

      ```bash
      DKP_REPO=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F':' '{print $1}')
      DKP_IMG=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image')

      d8 k run dkp-image --image=$DKP_IMG --command sleep -- infinity
      d8 k wait --for=condition=ready pod/dkp-image --timeout=300s

      DECKHOUSE_KUBE_RBAC_PROXY=$(d8 k exec dkp-image -- cat /deckhouse/modules/images_digests.json | jq -r ".common.kubeRbacProxy")
      DECKHOUSE_INIT=$(d8 k exec dkp-image -- cat /deckhouse/modules/images_digests.json | jq -r ".deckhouse.init")

      # Либо
      DECKHOUSE_KUBE_RBAC_PROXY=$(d8 k exec dkp-image -- cat /deckhouse/candi/images_digests.json | jq -r ".common.kubeRbacProxy")
      DECKHOUSE_INIT=$(d8 k exec dkp-image -- cat /deckhouse/candi/images_digests.json | jq -r ".deckhouse.init")

      d8 k delete pod dkp-image
      ```

      Применение для образа `kube-rbac-proxy`:

      ```bash
      d8 k -n d8-system set image deployment/deckhouse kube-rbac-proxy=$DKP_REPO@$DECKHOUSE_KUBE_RBAC_PROXY
      ```

      Применение для образа `initContainer`:

      ```bash
      d8 k -n d8-system set image deployment/deckhouse init-downloaded-modules=$DKP_REPO@$DECKHOUSE_INIT
      ```

{% endcapture %}

{% capture take_care_of_the_queue %}
1. Убедитесь в выполнении всех задач в очередях DKP, прежде чем продолжить процесс переключения:

   {{ wait_queue | regex_replace: "^", "   " }}
{% endcapture %}

Редакции DKP различаются набором модулей, поддерживаемых версий Kubernetes и функциональными возможностями. Важно понимать, какие изменения в функциональности произойдут при переключении, какие возможности станут недоступными. Это поможет вам подготовиться к процессу переключения.

Сравнение редакций DKP по составу модулей представлено странице [«Сравнение редакций»](../../../reference/revision-comparison.html).

Что необходимо учесть перед переключением:

{% tabs step1 %}
{% tab "На DKP CE" %}

{{
   take_care_of_the_internal_modules
   | regex_replace: "(?m)^[ \t]*<!REMOVE_FOR_CE>.+?<!/REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "ce"
}}

{{
   take_care_of_the_external_modules
   | regex_replace: "(?m)^[ \t]*<!REMOVE_FOR_CE>.+?<!/REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "ce"
}}

{{ take_care_of_the_queue }}
{% endtab %}

{% tab "На DKP BE" %}
{{
   take_care_of_the_internal_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "be"
}}

{{
   take_care_of_the_external_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "be"
}}

{{ take_care_of_the_queue }}
{% endtab %}

{% tab "На DKP SE" %}
{{
   take_care_of_the_internal_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "se"
}}

{{
   take_care_of_the_external_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "se"
}}

{{ take_care_of_the_queue }}
{% endtab %}

{% tab "На DKP SE+" %}
{{
   take_care_of_the_internal_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "se-plus"
}}

{{
   take_care_of_the_external_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "se-plus"
}}

{{ take_care_of_the_queue }}
{% endtab %}

{% tab "На DKP EE" %}
{{
   take_care_of_the_internal_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "ee"
}}

{{
   take_care_of_the_external_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "ee"
}}

{{ take_care_of_the_queue }}
{% endtab %}

{% tab "На DKP CSE" %}
1. При переключении на DKP CSE возможна временная недоступность компонентов кластера.
1. Переключение на DKP CSE возможно только с DKP EE (Enterprise Edition). Переключение поддерживается только **между одинаковыми минорными версиями** DKP. Например, с DKP EE 1.67.x на DKP CSE 1.67.x.

   При необходимости, выполните обновление DKP EE до соответствующей минорной версии и последней патч-версии.

   Актуальные патч-версии DKP CSE: `v1.58.2`, `v1.64.1`, `v1.67.4`, `v1.73.0`. Также, информацию о доступных версиях DKP CSE можно получить в разделе [Обновления DKP Certified Security Edition](https://deckhouse.ru/products/kubernetes-platform/certified-security-edition/updates/) на официальном сайте.

1. Убедитесь, что версия Kubernetes, используемая в кластере, поддерживается в желаемой версии DKP CSE:
   - DKP CSE 1.58 и 1.64 поддерживает Kubernetes версии 1.27;
   - DKP CSE 1.67 поддерживает Kubernetes версий 1.27 и 1.29.

   При необходимости, обновите версию Kubernetes в кластере до поддерживаемой:
   - Выполните команду:

     ```shell
     d8 platform edit cluster-configuration
     ```

   - Измените параметр `kubernetesVersion` на необходимое значение, например, `"1.29"` (в кавычках) для Kubernetes 1.29.
   - Сохраните изменения. Узлы кластера начнут последовательно обновляться.
   - Дождитесь окончания обновления. Отслеживать ход обновления можно с помощью команды `d8 k get no`. Обновление считается завершенным, когда в выводе команды у каждого узла кластера в колонке `VERSION` появится обновленная версия.

{{
   take_care_of_the_internal_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "registry.deckhouse.ru", "registry-cse.deckhouse.ru"
   | regex_replace: "\$NEW_EDITION", "cse"
}}

{{
   take_care_of_the_external_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "registry.deckhouse.ru", "registry-cse.deckhouse.ru"
   | regex_replace: "\$NEW_EDITION", "cse"
}}

{{ take_care_of_the_queue }}
{% endtab %}
{% endtabs %}

## Переключение редакции

### Выбор способа переключения

При выборе способа переключения редакции учитывайте то, каким образом в кластере организована работа с хранилищем образов контейнеров DKP.

Существует два способа работы с хранилищем образов контейнеров DKP:

- С использованием модуля [`registry`](/modules/registry/) — **(рекомендованный способ)**, конфигурация работы с хранилищем образов DKP задана в секции [`registry`](/modules/deckhouse/configuration.html#parameters-registry) параметров модуля `deckhouse` (ModuleConfig `deckhouse`). Это обеспечивает более плавный процесс перехода и автоматическую проверку наличия необходимых образов. Если в кластере используется этот способ работы с хранилищем образов контейнеров DKP, для переключения редакции воспользуйтесь разделом [«Переключение с помощью модуля registry»](#переключение-с-помощью-модуля-registry).

- Без использования модуля `registry` — конфигурация работы с хранилищем образов DKP задаётся при установке кластера [в `InitConfiguration`](../../../reference/api/cr.html#initconfiguration-deckhouse-imagesrepo), параметр [`registry.mode`](/modules/deckhouse/configuration.html#parameters-registry-mode) модуля `deckhouse` (ModuleConfig `deckhouse`) установлен в `Unmanaged`, параметр [`registry.unmanaged`](/modules/deckhouse/configuration.html#parameters-registry-unmanaged) модуля `deckhouse` не задан.

  Этот способ — единственный доступный для managed Kubernetes-кластеров, где control plane управляется провайдером облачных услуг, а не DKP (например, Amazon EKS, Azure AKS, Google GKE и др.).

  Если в кластере используется этот способ работы с хранилищем образов контейнеров DKP, для переключения редакции воспользуйтесь разделом [«Переключение без использования модуля registry»](#переключение-без-использования-модуля-registry).

Перед выполнением дальнейших шагов выполните подготовительные действия, описанные в разделе [«Подготовка к переключению»](#подготовка-к-переключению).

{% capture bashible_sync_wait %}
Дождитесь синхронизации сервиса bashible (значение в колонке `UPTODATE` у NodeGroup должно совпадать с `NODES`):

```shell
d8 k get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate
```

В логе bashible должно быть `Configuration is in sync, nothing to do`:

```shell
journalctl -u bashible -n 5
```

{% endcapture %}

{% capture check_old_pods_unmanaged %}

```shell
d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[] | select(.image | contains("deckhouse.ru/deckhouse/<КОД_ПРЕДЫДУЩЕЙ_РЕДАКЦИИ>"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
```

{% endcapture %}

{% capture enable_chrony_cse %}

```shell
d8 system module enable chrony
```

{% endcapture %}

### Переключение с помощью модуля registry

{% alert level="warning" %}
Не подходит для managed Kubernetes (EKS, AKS, GKE) и для DKP CSE **ниже** 1.73.
{% endalert %}

{% capture change_registry_mc_deckhouse_unmanaged %}

```yaml
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
<!REMOVE_FOR_CE>
        license: <ЛИЦЕНЗИОННЫЙ_КЛЮЧ>
<!/REMOVE_FOR_CE>
        checkMode: <РЕЖИМ_ПРОВЕРКИ>
        imagesRepo: <REGISTRY_HOST>/deckhouse/<КОД_РЕДАКЦИИ>
        scheme: HTTPS
```

{% endcapture %}

{% capture registry_status_cmd %}

```shell
d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml) | {"conditions": [.conditions[] | select(.type == "Ready" or .type == "RegistryContainsRequiredImages")]}'
```

{% endcapture %}

{% capture registry_status_example %}

```yaml
conditions:
  - lastTransitionTime: "2026-05-05T13:53:23Z"
    message: |-
      Mode: Default
      <REGISTRY_HOST>: all 182 items are checked
    reason: Ready
    status: "True"
    type: RegistryContainsRequiredImages
  - lastTransitionTime: "2026-05-05T13:54:49Z"
    message: ""
    reason: ""
    status: "True"
    type: Ready
```

{% endcapture %}

1. Убедитесь, что в кластере используется модуль registry. В moduleConfig `deckhouse` должны быть указаны параметры registry предыдущей редакции DKP в `Unmanaged` режиме работы. Если это не так, выполните [миграцию на использование модуля registry](../registry/managing-interaction.html#миграция-на-формат-управления-настройками-хранилища-образов-с-использованием-модуля-registry).

   После выполнения миграции, в moduleConfig `deckhouse` должны появиться параметры registry:

   {% tabs switch-registry-edition-1 %}
   {% tab "DKP CE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<КОД_РЕДАКЦИИ>", "<КОД_ПРЕДЫДУЩЕЙ_РЕДАКЦИИ>"
         | regex_replace: "<РЕЖИМ_ПРОВЕРКИ>", "Default"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.ru"
         | regex_replace: "(?m)<!REMOVE_FOR_CE>.+?<!/REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP BE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<КОД_РЕДАКЦИИ>", "<КОД_ПРЕДЫДУЩЕЙ_РЕДАКЦИИ>"
         | regex_replace: "<РЕЖИМ_ПРОВЕРКИ>", "Default"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.ru"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP SE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<КОД_РЕДАКЦИИ>", "<КОД_ПРЕДЫДУЩЕЙ_РЕДАКЦИИ>"
         | regex_replace: "<РЕЖИМ_ПРОВЕРКИ>", "Default"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.ru"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP SE+" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<КОД_РЕДАКЦИИ>", "<КОД_ПРЕДЫДУЩЕЙ_РЕДАКЦИИ>"
         | regex_replace: "<РЕЖИМ_ПРОВЕРКИ>", "Default"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.ru"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP EE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<КОД_РЕДАКЦИИ>", "<КОД_ПРЕДЫДУЩЕЙ_РЕДАКЦИИ>"
         | regex_replace: "<РЕЖИМ_ПРОВЕРКИ>", "Default"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.ru"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP CSE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<КОД_РЕДАКЦИИ>", "<КОД_ПРЕДЫДУЩЕЙ_РЕДАКЦИИ>"
         | regex_replace: "<РЕЖИМ_ПРОВЕРКИ>", "Default"
         | regex_replace: "<REGISTRY_HOST>", "registry-cse.deckhouse.ru"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}
   {% endtabs %}

   В статусе переключения не должно быть ошибок:

   {{ registry_status_cmd | regex_replace: "^", "   " }}

   Пример успешного вывода:

   {% tabs switch-registry-status-example-3 %}
   {% tab "CE/BE/SE/SE+/EE" %}
      {{
         registry_status_example
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.ru"
         | regex_replace: "^", "   "
      }}
   {% endtab %}

   {% tab "CSE" %}
      {{
         registry_status_example
         | regex_replace: "<REGISTRY_HOST>", "registry-cse.deckhouse.ru"
         | regex_replace: "^", "   "
      }}
   {% endtab %}
   {% endtabs %}

1. В ModuleConfig [`deckhouse`](/modules/deckhouse/configuration.html#parameters-registry) укажите `imagesRepo` целевой редакции и `checkMode: Relax`:

   Выполните команду для редактирования ModuleConfig `deckhouse`:

   ```shell
   d8 k edit moduleconfig deckhouse
   ```

   Выберите пример для вашей редакции:

   {% tabs switch-registry-edition %}
   {% tab "DKP CE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<КОД_РЕДАКЦИИ>", "ce"
         | regex_replace: "<РЕЖИМ_ПРОВЕРКИ>", "Relax"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.ru"
         | regex_replace: "(?m)<!REMOVE_FOR_CE>.+?<!/REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP BE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<КОД_РЕДАКЦИИ>", "be"
         | regex_replace: "<РЕЖИМ_ПРОВЕРКИ>", "Relax"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.ru"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP SE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<КОД_РЕДАКЦИИ>", "se"
         | regex_replace: "<РЕЖИМ_ПРОВЕРКИ>", "Relax"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.ru"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP SE+" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<КОД_РЕДАКЦИИ>", "se-plus"
         | regex_replace: "<РЕЖИМ_ПРОВЕРКИ>", "Relax"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.ru"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP EE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<КОД_РЕДАКЦИИ>", "ee"
         | regex_replace: "<РЕЖИМ_ПРОВЕРКИ>", "Relax"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.ru"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP CSE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<КОД_РЕДАКЦИИ>", "cse"
         | regex_replace: "<РЕЖИМ_ПРОВЕРКИ>", "Relax"
         | regex_replace: "<REGISTRY_HOST>", "registry-cse.deckhouse.ru"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}
   {% endtabs %}

{{ take_care_deckhuse_imagepullbackoff }}

1. Дождитесь переключения.

   Проверка статуса переключения:

   {{ registry_status_cmd | regex_replace: "^", "   " }}

   Пример успешного вывода:

   {% tabs switch-registry-status-example-2 %}
   {% tab "CE/BE/SE/SE+/EE" %}
      {{
         registry_status_example
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.ru"
         | regex_replace: "^", "   "
      }}
   {% endtab %}
   {% tab "CSE" %}
      {{
         registry_status_example
         | regex_replace: "<REGISTRY_HOST>", "registry-cse.deckhouse.ru"
         | regex_replace: "^", "   "
      }}
   {% endtab %}
   {% endtabs %}

1. Верните `checkMode` в `Default`:

   ```shell
   d8 k patch moduleconfig deckhouse --type=json -p='[{"op": "replace", "path": "/spec/settings/registry/unmanaged/checkMode", "value": "Default"}]'
   ```

1. Снова проверьте статус переключения.

   Проверка статуса:

   {{ registry_status_cmd | regex_replace: "^", "   " }}

   Пример успешного вывода:

   {% tabs switch-registry-status-example %}
   {% tab "CE/BE/SE/SE+/EE" %}{{ registry_status_example | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.ru" | regex_replace: "^", "   " }}{% endtab %}
   {% tab "CSE" %}{{ registry_status_example | regex_replace: "<REGISTRY_HOST>", "registry-cse.deckhouse.ru" | regex_replace: "^", "   " }}{% endtab %}
   {% endtabs %}

1. Проверьте наличие подов с ошибками загрузки образов:

   ```shell
   d8 k get pods -A | awk 'NR==1 || /^d8-/' | grep -E 'ImagePullBackOff|ErrImagePull'
   ```

   Для каждого проблемного модуля **на всех master-узлах** выполните следующие команды, указав имя модуля:

   ```shell
   rm -rf /var/lib/deckhouse/downloaded/<ИМЯ_МОДУЛЯ>/
   d8 k rollout restart deploy -n d8-system deckhouse
   ```

1. Проверьте поды с образами из хранилища образов контейнеров для старой редакции:

   {{ check_old_pods_unmanaged }}

1. **Только для DKP CSE** — включите модуль `chrony`:

   {{ enable_chrony_cse | regex_replace: "^", "   " }}

### Переключение без использования модуля registry

{% capture alert_additional_registry %}
{% alert level="info" %}
Если необходимо добавить конфигурации для дополнительного registry в containerd, воспользуйтесь инструкцией из раздела [«Как добавить конфигурацию для дополнительного registry в containerd»](/modules/node-manager/faq.html#как-добавить-конфигурацию-для-дополнительного-registry).
{% endalert %}
{% endcapture %}

{% capture ngc_auth_registry %}
{{ alert_additional_registry }}

```shell
AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
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

{% endcapture %}

{% capture change_registry_helper_ce %}

```shell
DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --new-deckhouse-tag=$DECKHOUSE_VERSION registry.deckhouse.ru/deckhouse/ce
```

{% endcapture %}

{% capture change_registry_helper_commercial %}

```shell
DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
DOCKER_CONFIG_JSON=$(echo -n "{\"auths\": {\"registry.deckhouse.ru\": {\"username\": \"license-token\", \"password\": \"${LICENSE_TOKEN}\", \"auth\": \"${AUTH_STRING}\"}}}" | base64 -w 0)
d8 k --as system:sudouser -n d8-cloud-instance-manager patch secret deckhouse-registry --type merge --patch="{\"data\":{\".dockerconfigjson\":\"$DOCKER_CONFIG_JSON\"}}"
d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user=license-token --password=$LICENSE_TOKEN --new-deckhouse-tag=$DECKHOUSE_VERSION registry.deckhouse.ru/deckhouse/$NEW_EDITION
```

{% endcapture %}

{% capture ngc_cleanup_registry %}

```shell
d8 k delete ngc containerd-$NEW_EDITION-config.sh
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
d8 k delete ngc del-temp-config.sh
```

{% endcapture %}

{% capture ngc_auth_cse %}
{{ alert_additional_registry }}

```shell
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

{% endcapture %}

{% capture cse_digests_from_pod %}

```shell
d8 k run cse-image --image=registry-cse.deckhouse.ru/deckhouse/cse/install:$DECKHOUSE_VERSION --command sleep -- infinity
d8 k wait --for=condition=ready pod/cse-image --timeout=300s
CSE_SANDBOX_IMAGE=$(d8 k exec cse-image -- cat deckhouse/candi/images_digests.json | grep pause | grep -oE 'sha256:\w*')
CSE_K8S_API_PROXY=$(d8 k exec cse-image -- cat deckhouse/candi/images_digests.json | grep kubernetesApiProxy | grep -oE 'sha256:\w*')
CSE_DECKHOUSE_KUBE_RBAC_PROXY=$(d8 k exec cse-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.kubeRbacProxy")
```

{% endcapture %}

{% capture cse_set_image_158 %}

```shell
d8 k -n d8-system set image deployment/deckhouse kube-rbac-proxy=registry-cse.deckhouse.ru/deckhouse/cse@$CSE_DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry-cse.deckhouse.ru/deckhouse/cse:$DECKHOUSE_VERSION
```

{% endcapture %}

{% capture cse_set_image_164_plus %}

```shell
CSE_DECKHOUSE_INIT_CONTAINER=$(d8 k exec cse-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.init")
d8 k -n d8-system set image deployment/deckhouse init-downloaded-modules=registry-cse.deckhouse.ru/deckhouse/cse@$CSE_DECKHOUSE_INIT_CONTAINER kube-rbac-proxy=registry-cse.deckhouse.ru/deckhouse/cse@$CSE_DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry-cse.deckhouse.ru/deckhouse/cse:$DECKHOUSE_VERSION
```

{% endcapture %}

{% capture cse_cleanup %}

```shell
d8 k delete ngc containerd-cse-config.sh cse-set-sha-images.sh
d8 k delete pod cse-image --ignore-not-found
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
    rm -f /etc/containerd/conf.d/cse-registry.toml /etc/containerd/conf.d/cse-sandbox.toml
EOF
d8 k delete ngc del-temp-config.sh
```

{% endcapture %}

{% alert level="warning" %}
Перед применением, убедитесь, что модуль registry не используется в кластере. В moduleConfig `deckhouse` должны отсутствовать параметры registry. Модуль `registry` должен быть выключен. Если это не так, выполните [миграция на устаревший формат управления настройками registry](../registry/managing-interaction.html#миграция-на-устаревший-формат-управления-настройками-хранилища-образов-компонентов-dkp-без-модуля-registry).
{% endalert %}

Выберите целевую редакцию:

{% tabs switch-without-registry %}
{% tab "DKP CE" %}
1. Переключите хранилище образов контейнеров:

   {{ change_registry_helper_ce | regex_replace: "^", "   " }}

1. Дождитесь готовности DKP:

   {{ wait_queue | regex_replace: "^", "   " }}

1. Проверьте, не остались ли использоваться образы от предыдущей редакции (укажите код **предыдущей** редакции):

   {{ check_old_pods_unmanaged | regex_replace: "^", "   " }}
{% endtab %}
{% tab "DKP BE" %}
1. Выполните команду для указания данных аутентификации в хранилище образов:

   {{ ngc_auth_registry | regex_replace: "\$NEW_EDITION", "be" | regex_replace: "^", "   " }}

   {{ bashible_sync_wait | regex_replace: "^", "   " }}

1. Переключите хранилище образов контейнеров:

   {{ change_registry_helper_commercial | regex_replace: "\$NEW_EDITION", "be" | regex_replace: "^", "   " }}

1. Дождитесь готовности DKP:

   {{ wait_queue | regex_replace: "^", "   " }}

1. Проверьте, не остались ли использоваться образы от предыдущей редакции (укажите код **предыдущей** редакции):

   {{ check_old_pods_unmanaged | regex_replace: "^", "   " }}

1. Выполните очистку:

   {{ ngc_cleanup_registry | regex_replace: "\$NEW_EDITION", "be" | regex_replace: "^", "   " }}
{% endtab %}
{% tab "DKP SE" %}
1. Выполните команду для указания данных аутентификации в хранилище образов:

   {{ ngc_auth_registry | regex_replace: "\$NEW_EDITION", "se" | regex_replace: "^", "   " }}

   {{ bashible_sync_wait | regex_replace: "^", "   " }}

1. Переключите хранилище образов контейнеров:

   {{ change_registry_helper_commercial | regex_replace: "\$NEW_EDITION", "se" | regex_replace: "^", "   " }}

1. Дождитесь готовности DKP:

   {{ wait_queue | regex_replace: "^", "   " }}

1. Проверьте, не остались ли использоваться образы от предыдущей редакции (укажите код **предыдущей** редакции):

   {{ check_old_pods_unmanaged | regex_replace: "^", "   " }}

1. Выполните очистку:

   {{ ngc_cleanup_registry | regex_replace: "\$NEW_EDITION", "se" | regex_replace: "^", "   " }}
{% endtab %}
{% tab "DKP SE+" %}
1. Выполните команду для указания данных аутентификации в хранилище образов:

   {{ ngc_auth_registry | regex_replace: "\$NEW_EDITION", "se-plus" | regex_replace: "^", "   " }}

   {{ bashible_sync_wait | regex_replace: "^", "   " }}

1. Переключите хранилище образов контейнеров:

   {{ change_registry_helper_commercial | regex_replace: "\$NEW_EDITION", "se-plus" | regex_replace: "^", "   " }}

1. Дождитесь готовности DKP:

   {{ wait_queue | regex_replace: "^", "   " }}

1. Проверьте, не остались ли использоваться образы от предыдущей редакции (укажите код **предыдущей** редакции):

   {{ check_old_pods_unmanaged | regex_replace: "^", "   " }}

1. Выполните очистку:

   {{ ngc_cleanup_registry | regex_replace: "\$NEW_EDITION", "se-plus" | regex_replace: "^", "   " }}
{% endtab %}
{% tab "DKP EE" %}
1. Выполните команду для указания данных аутентификации в хранилище образов:

   {{ ngc_auth_registry | regex_replace: "\$NEW_EDITION", "ee" | regex_replace: "^", "   " }}

   {{ bashible_sync_wait | regex_replace: "^", "   " }}

1. Переключите хранилище образов контейнеров:

   {{ change_registry_helper_commercial | regex_replace: "\$NEW_EDITION", "ee" | regex_replace: "^", "   " }}

1. Дождитесь готовности DKP:

   {{ wait_queue | regex_replace: "^", "   " }}

1. Проверьте, не остались ли использоваться образы от предыдущей редакции (укажите код **предыдущей** редакции):

   {{ check_old_pods_unmanaged | regex_replace: "^", "   " }}

1. Выполните очистку:

   {{ ngc_cleanup_registry | regex_replace: "\$NEW_EDITION", "ee" | regex_replace: "^", "   " }}
{% endtab %}
{% tab "DKP CSE" %}
1. Укажите версию DKP CSE, которую вы хотите использовать:

   {% tabs cse-switch-deckhouse-version %}
   {% tab "CSE 1.58" %}

   ```shell
   DECKHOUSE_VERSION=v1.58.2
   ```

   {% endtab %}
   {% tab "CSE 1.64" %}

   ```shell
   DECKHOUSE_VERSION=v1.64.1
   ```

   {% endtab %}
   {% tab "CSE 1.67" %}

   ```shell
   DECKHOUSE_VERSION=v1.67.4
   ```

   {% endtab %}
   {% tab "CSE 1.73" %}

   ```shell
   DECKHOUSE_VERSION=v1.73.3
   ```

   {% endtab %}
   {% endtabs %}

1. Выполните команду для указания данных аутентификации в хранилище образов:

   {{ ngc_auth_cse | regex_replace: "^", "   " }}

   {{ bashible_sync_wait | regex_replace: "^", "   " }}

1. Получите дайджесты образов (переменные будут использоваться в следующих шагах):

   {{ cse_digests_from_pod | regex_replace: "^", "   " }}

1. Настройте образы на узлах:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: cse-set-sha-images.sh
   spec:
     nodeGroups: ['*']
     bundles: ['*']
     weight: 50
     content: |
       _on_containerd_config_changed() { bb-flag-set containerd-need-restart }
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

   {{ bashible_sync_wait | regex_replace: "^", "   " }}

1. Обновите данные аутентификации для доступа к хранилищу образов:

   ```shell
   AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
   d8 k -n d8-system create secret generic deckhouse-registry \
     --from-literal=".dockerconfigjson"="{\"auths\": { \"registry-cse.deckhouse.ru\": { \"username\": \"license-token\", \"password\": \"$LICENSE_TOKEN\", \"auth\": \"$AUTH_STRING\" }}}" \
     --from-literal="address"=registry-cse.deckhouse.ru \
     --from-literal="path"=/deckhouse/cse \
     --from-literal="scheme"=https \
     --type=kubernetes.io/dockerconfigjson \
     --dry-run=client -o yaml | d8 k -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- d8 k replace -f -
   ```

1. Смените образ DKP CSE:

   {% tabs cse-set-deckhouse-image %}
   {% tab "CSE 1.58" %}
   {{ cse_set_image_158 | regex_replace: "^", "   " }}
   {% endtab %}
   {% tab "CSE 1.64 / 1.67 / 1.73" %}
   {{ cse_set_image_164_plus | regex_replace: "^", "   " }}
   {% endtab %}
   {% endtabs %}

1. Дождитесь готовности DKP:

   {{ wait_queue | regex_replace: "^", "   " }}

1. Проверьте, не остались ли использоваться образы от DKP EE:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[] | select(.image | contains("deckhouse.ru/deckhouse/ee"))) | .metadata.namespace + "\t" + .metadata.name' | sort -u
   ```

1. Включите модуль `chrony`:

   {{ enable_chrony_cse | regex_replace: "^", "   " }}

1. Выполните очистку:

   {{ cse_cleanup | regex_replace: "^", "   " }}
{% endtab %}
{% endtabs %}
