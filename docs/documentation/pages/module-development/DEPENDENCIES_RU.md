---
title: "Зависимости модуля Deckhouse Kubernetes Platform"
permalink: ru/module-development/dependencies/
lang: ru
---

В этом разделе описаны зависимости, которые могут быть установлены для модуля.

Зависимости — это набор условий (требований), которые должны выполняться, чтобы Deckhouse Kubernetes Platform (DKP) мог запустить модуль.

DKP поддерживает следующие зависимости для модуля:

- [зависимость от версии Deckhouse Kubernetes Platform](#зависимость-от-версии-deckhouse-kubernetes-platform);
- [зависимость от версии Kubernetes](#зависимость-от-версии-kubernetes);
- [зависимость от версии других модулей](##зависимость-от-версии-других-модулей).

## Зависимость от версии Deckhouse Kubernetes Platform

Эта зависимость определяет минимальную или максимальную версию DKP, с которой совместим модуль.

Пример настройки зависимости модуля от версии DKP 1.61 и выше в файле `module.yaml`:

```yaml
name: test
weight: 901
requirements:
    deckhouse: ">= 1.61"
```

{% alert level="info" %}
Для тестирования можно задать переменную окружения `TEST_EXTENDER_DECKHOUSE_VERSION`, чтобы симулировать желаемую версию DKP.
{% endalert %}

Зависимость проверяется в следующих случаях:

1. **При установке или обновлении модуля.**
   Если версия DKP не соответствует требованиям, указанным в зависимостях модуля релиза, его установка или обновление не будут выполнены.

   Пример ресурса ModuleRelease, когда версия DKP не соответствует требованиям модуля:

   ```console
   root@dev-master-0:~# kubectl get mr
   ```

   Выводимая информация:

   ```text
   NAME                     PHASE        UPDATE POLICY   TRANSITIONTIME   MESSAGE
   test-v0.8.3              Pending      test-alpha      2m30s            requirements are not satisfied: current deckhouse version is not suitable: 1.0.0 is less than or equal to v1.64.0 
   ```

1. **При обновлении DKP.**  
   Проверяется, соответствует ли новая версия DKP зависимостям установленных и активных модулей. Если хотя бы один модуль несовместим с новой версией, обновление DKP не выполнится.

   Пример ресурса DeckhouseRelease, когда версия DKP не соответствует требованиям модуля:

   ```console
   root@dev-master-0:~# kubectl get deckhousereleases.deckhouse.io
   ```

   Выводимая информация:

   ```text
   NAME                     PHASE         TRANSITIONTIME   MESSAGE
   v1.73.3                  Skipped       74m
   v1.73.4                  Pending       2m13s            requirements of test are not satisfied: v1.73.4 deckhouse version is not suitable: v1.73.4 is greater than or equal to v1.73.4
   ```

1. **При первичном анализе модулей.**
   Проверяются текущая версия DKP и зависимости уже установленных модулей. Если обнаружено несоответствие, модуль будет отключён.

## Зависимость от версии Kubernetes

Эта зависимость определяет минимальную или максимальную версию Kubernetes, с которой совместим модуль.

Пример настройки зависимости от Kubernetes 1.28 и выше в файле `module.yaml`:

```yaml
name: test
weight: 901
requirements:
    kubernetes: ">= 1.28"
```

{% alert level="info" %}
Для тестирования можно задать переменную окружения `TEST_EXTENDER_KUBERNETES_VERSION`, чтобы симулировать желаемую версию Kubernetes.
{% endalert %}

Зависимость проверяется в следующих случаях:

1. **При установке или обновлении модуля.**
   Если версия Kubernetes не соответствует требованиям, указанным в зависимостях модуля релиза, установка или обновление не будут выполнены.

   Пример ресурса ModuleRelease, когда версия Kubernetes не соответствует требованиям модуля:

   ```console
   root@dev-master-0:~# kubectl get modulereleases.deckhouse.io
   ```

   Выводимая информация:

   ```text
   NAME                          PHASE        UPDATE POLICY   TRANSITIONTIME   MESSAGE
   test-v0.8.2                   Pending      test-alpha      24m              requirements are not satisfied: current kubernetes version is not suitable: 1.29.6 is less than or equal to 1.29
   virtualization-v.0.0.0-dev4   Deployed      deckhouse      142d
   ```

1. **При обновлении версии Kubernetes.**
   Проверяются зависимости активных модулей, и если хотя бы один модуль несовместим с новой версией Kubernetes, изменение версии не будет принято.

   Пример вывода при несовместимости модуля с новой версией Kubernetes:

   ```console
   root@dev-master-0:~# d8 platform edit cluster-configuration
   ```

   Выводимая информация:

   ```text
   Save cluster-configuration back to the Kubernetes cluster
   Update cluster-configuration secret
   Attempt 1 of 5 |
           Update cluster-configuration secret failed, next attempt will be in 5s"
           Error: admission webhook "kubernetes-version.deckhouse-webhook.deckhouse.io" denied the request: requirements of test are not satisfied: 1.27 kubernetes version is not suitable: 1.27.0 is less than or equal to 1.28
   ```

1. **При первичном анализе модулей.**
   Если версия Kubernetes не соответствует зависимостям уже установленных модулей, DKP отключит такие модули.

1. **При обновлении DKP.**
   Проверяется значение версии Kubernetes, установленной по умолчанию для DKP, если оно несовместимо с активными модулями, обновление DKP не будет выполнено.

   Пример ресурса DeckhouseRelease, когда версия Kubernetes не соответствует требованиям модуля:

   ```console
   root@dev-master-0:~# kubectl get deckhousereleases.deckhouse.io
   ```

   Выводимая информация:

   ```text
   NAME                     PHASE         TRANSITIONTIME   MESSAGE
   v1.73.3                  Pending       7s              requirements of test are not satisfied: 1.27 kubernetes version is not suitable: 1.27.0 is less than or equal to 1.28            
   ```

## Зависимость от версии других модулей

Зависимости от версии других модулей описывают условия включения, обновления и выключения модуля.
Модуль в Deckhouse Kubernetes Platform может иметь обязательные и необязательные зависимости от версий других модулей.

### Обязательные зависимости

Обязательная зависимость определяет список **включенных** модулей и их версии, которые необходимы для работы модуля.

{% alert level="info" %}
Версия встроенного модуля DKP считается равной версии DKP.
{% endalert %}

Если необходимо указать, чтобы какой-то модуль был просто включен, не важно какой версии, то можно использовать следующий синтаксис (на примере модуля `user-authn`):

```yaml
requirements:
  modules:
    user-authn: ">= 0.0.0"
```

Пример настройки зависимости от трех модулей:

```yaml
name: hello-world
requirements:
  modules:
    ingress-nginx: '> 1.67.0'
    node-local-dns: '>= 0.0.0'
    operator-trivy: '> v1.64.0'
```

### Необязательные зависимости

{% alert level="danger" %}
Необязательные зависимости для модулей доступны для Deckhouse Kubernetes Platform, начиная с версии 1.73.
Если вы планируете использовать их для какого-то модуля, [задайте для него зависимость от версии DKP](#зависимость-от-версии-deckhouse-kubernetes-platform) 1.73 и выше.
{% endalert %}

Необязательная зависимость используется, когда модуль работает самостоятельно, но может использоваться совместно с другим модулем, **если он включён**.

{% alert level="info" %}
Необязательные зависимости могут влиять на возможность включения, выключения и обновления обоих модулей: зависимого и того, от которого он зависит.
{% endalert %}

Чтобы указать, что зависимость является необязательной, добавьте `!optional` к строке ограничения версии модуля, от которого может зависеть целевой модуль:

```yaml
name: prometheus
requirements:
  modules:
    test: ">v0.22.1 !optional"
```

> В описании и примерах ниже:
>
> - `prometheus` — целевой модуль, для которого задается необязательная зависимость;
> - `test` — модуль, совместно с которым может использоваться целевой.

#### Ограничения по включению и выключению prometheus при наличии необязательной зависимости от test

При наличии необязательной зависимости от версии другого модуля целевой имеет следующие ограничения по включению и выключению:

1. Если в кластере не включен `test`, `prometheus` может быть включен.

   **Пример:** `test` выключен + для `prometheus` задано необязательное требование `test: ">v0.22.1 !optional"` → `prometheus` будет включен, требование пропускается.

1. Если в кластере уже включен `test`, включение `prometheus` с `requirements` возможно только, если в `requirements` указаны требования, которым соответствует текущая версия `test`.

   **Пример:** в кластере включен `test` версии `v0.21.1` + для `prometheus` задано необязательное требование `test: >v0.22.1 !optional` → установка/включение `prometheus` завершится ошибкой о несоответствии зависимости (текущая версия `test` не соответствует `requirements`).

1. Если `test` будет выключен в кластере, `prometheus` останется включенным.

   **Пример:** в кластере включен `test` + для `prometheus` задано необязательное требование `test: >v0.22.1 !optional` → выключение `test` пройдет успешно, `prometheus` выключен не будет.

#### Ограничения по обновлению prometheus при наличии необязательной зависимости от test

При наличии необязательной зависимости от версии другого модуля целевой модуль имеет следующие ограничения по обновлению:

1. `prometheus` может быть обновлен, даже если в кластере нет модуля `test`.

   **Пример:** `test` выключен + для `prometheus` задано необязательное требование `test: ">v0.22.1 !optional"` → `prometheus` будет обновлен.

1. Обновление для `prometheus` будет заблокировано, пока в кластере не обновится `test` указанной в `requirements` версии.

   **Пример:** `test` выключен + для `prometheus` задано необязательное требование `test: ">v0.22.1 !optional"` → `prometheus` не будет обновлен, пока не обновится `test`.

#### Ограничения по включению test, версия которого указана в зависимостях prometheus

Если модуль `prometheus` включен, невозможно включить `test`, версия которого не соответствует выражению, указанному в `requirements` для `prometheus`.

**Пример:** `prometheus` включен + для `prometheus` задано необязательное требование `test: ">v0.22.1 !optional"` → попытка включения `test v0.21.1` завершится ошибкой о несоответствии зависимости (версия `test v0.21.1` не соответствует условию в `requirements` для `prometheus`).

#### Ограничения по обновлению test, версия которого указана в зависимостях prometheus

Если `prometheus` и `test` включены в кластере, обновление `test` возможно только на версию которая соответствует требованиям, указанным в `requirements` для `prometheus`.

**Пример:** Модули `prometheus` и `test` включены в кластере + для `prometheus` задано необязательное требование `test: "=v0.22.1 !optional"` + попытка обновления `test` до версии `0.23.1` → `test` не будет обновлен, т.к. требуемая версия не соответствует `requirements` для `prometheus`.

{% alert level="warning" %}
- Включение и отключение модулей может занимать больше времени из‑за дополнительных проверок экстендера.
- Известное ограничение: во время обработки модулей список включённых модулей может кратковременно быть пустым. В редких случаях это позволяет ошибочно пройти проверку опциональной зависимости. Если столкнулись, повторите операцию.
{% endalert %}
