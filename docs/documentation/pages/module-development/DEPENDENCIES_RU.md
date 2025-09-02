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

### Зависимость от версии Deckhouse Kubernetes Platform

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

### Зависимость от версии Kubernetes

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

### Зависимость от версии других модулей

Эта зависимость определяет список **включенных** модулей и их минимальные версии, которые необходимы для работы модуля. Версия встроенного модуля DKP считается равной версии DKP.

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
#### Необязательные требования модулей

Используйте, когда ваш модуль работает самостоятельно, но может интегрироваться с другим модулем, **если он включён**.

Чтобы указать, что требование является необязательным, добавьте `!optional` к строке ограничения версии:

```yaml
requirements:
  modules:
    test1: ">v0.22.1 !optional"
```

Правила:

- Если `test1` **включён**, его версия **должна** удовлетворять требованию.
- Если `test1` **выключен**, требование игнорируется, и модуль можно установить или обновить.
- Если позже включить `test1` с несовместимой версией, включение будет отклонено, а модуль останется выключенным.

Примеры:

- `test1` включен на `v0.21.1` + `>v0.22.1 !optional` → установка/включение модуля с требованием завершится ошибкой о несоответствии зависимости.
- `test1` выключен + `>v0.22.1 !optional` → установка проходит; требование пропускается.
- `test` выключен, `test1` включён на `v0.21.1` + `>v0.22.1 !optional` → включение `test` отклоняется.
- `test` включён с требованием на `test1`; попытка включить `test1` с неподходящей версией отклоняется, `test1` остаётся выключенным.

{% alert level="warning" %}
- Включение и отключение модулей может занимать больше времени из‑за дополнительных проверок экстендера.
- Известное ограничение: во время обработки модулей список включённых модулей может кратковременно быть пустым. В редких случаях это позволяет ошибочно пройти проверку опциональной зависимости. Если столкнулись, повторите операцию.
{% endalert %}
