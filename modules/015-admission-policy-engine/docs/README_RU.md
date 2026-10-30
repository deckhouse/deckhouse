---
title: "Модуль admission-policy-engine"
description: Модуль admission-policy-engine Deckhouse позволяет использовать в кластере Kubernetes политики безопасности согласно Kubernetes Pod Security Standards.
---

Модуль `admission-policy-engine` реализует поддержку admission-политик безопасности в кластере Kubernetes.

Admission-политики — это правила, которые применяются к объектам (например Pod и Service) в момент их создания и изменения в кластере (но не в процессе их работы), на основе информации, представленной в их манифесте. Эти политики направлены на формализацию параметров которые разрешены или запрещены в манифестах объектов.

В DP политики разделены на три категории:

- [Pod Security Standards](#pod-security-standards) — политики, реализующие требования соответствующих [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/).
- [Операционные политики](#операционные-политики) — политики для создания дополнительных требований к объектам, с помощью валидации значений параметров **не связанных напрямую** с безопасностью (например, список допустимых префиксов для образов контейнеров, политика скачивания образов, список необходимых проб для контейнеров и т.д.).
- [Политики безопасности](#политики-безопасности) — политики для создания дополнительных требований к объектам, с помощью валидации значений параметров, связанных с безопасностью (например, доступ контейнеров к IPC- или PID-пространству имен хоста, список привилегий для контейнеров и т.д.).

{% alert level="info" %}
Эти политики дополняют друг друга. Если для одного неймспейса применены несколько политик, выполняется валидация объектов по каждой из них. Если хоть одна политика будет нарушена, объект создан не будет.
{% endalert %}

Помимо политик, которые запрещают использование параметров, не соответствующих заданным требованиям, модуль поддерживает [ресурс SecurityPolicyException](#исключения-из-политик-безопасности), который позволяет создавать точечные исключения из проверок политик безопасности. С помощью этого ресурса можно разрешить использование отдельных параметров для конкретных подов или контейнеров, не изменяя политики безопасности, действующие для всего неймспейса.

## Особенности отображения сообщений о неудачной валидации объектов

В зависимости от способа создания подов есть особенности формирования сообщений от API о неудачной валидации (нарушении установленных политик):

- Если под создается напрямую, ошибка валидации возвращается в ответе от API о неудачной валидации (нарушении политики).
- Если поды создаются через Deployment, при этом поды потенциально нарушают политику, то создание такого Deployment будет отклонено.
- Если поды создаются через Deployment, который уже существовал до включения политики, то создаётся требуемое количество ReplicaSet, которые, в свою очередь, пытаются создать поды. В этом случае ошибка валидации не возвращается в ответе API, а отображается в событиях неймспейса или соответствующего ReplicaSet.

## Проверка на уровне контроллеров

Политики валидируют контроллеры, создающие поды, при операциях `CREATE` и `UPDATE`. Это обеспечивает ранний отклик при создании или обновлении рабочей нагрузки, до запуска подов, а также отображение запретов создания при развёртывании с помощью CI (в CI редко создаются поды, а ошибки создания подов от контроллеров чаще всего не отображаются).

{% alert level="warning" %}
По умолчанию контроллер, который нарушает политику с `enforcementAction: Deny`, получает предупреждение, а не запрет.
Поды, которые создаёт контроллер, по-прежнему запрещаются.
Модуль проверяет контроллеры отдельными ограничениями с именами `d8-controllers-<NAME>` и `enforcementAction: warn`.
О таких контроллерах сообщает алерт `ControllerPolicyViolation` — отдельно для каждой политики и неймспейса.
Это временное значение по умолчанию на период перехода к проверке контроллеров.
Чтобы уже сейчас проверять контроллеры с действием политики, установите параметр
[`controllerValidationEnforcement`](configuration.html#parameters-podsecuritystandards-controllervalidationenforcement) в значение `Policy`.
В одном из следующих релизов `Policy` станет значением по умолчанию.
{% endalert %}

Валидируются операции со следующими объектами:

| API-группа | Тип ресурса                        | Операции                     |
| ---------- | ---------------------------------- |------------------------------|
| (core)     | Pod                                | `CREATE`, `UPDATE`, `DELETE` |
| apps       | Deployment, StatefulSet, DaemonSet | `CREATE`, `UPDATE`, `DELETE` |
| (core)     | ReplicationController              | `CREATE`, `UPDATE`, `DELETE` |
| batch      | Job, CronJob                       | `CREATE`, `UPDATE`, `DELETE` |
| (core)     | pods/exec, pods/attach             | `CONNECT`                    |

Операция `DELETE` перехватывается, чтобы политика могла запретить удаление ресурса:
правило, которое проверяет `input.review.operation`, выполняется только если запрос доходит до Gatekeeper.

{% alert level="warning" %}
Вебхук работает с `failurePolicy: Fail`.
Пока Gatekeeper недоступен, для перечисленных выше типов ресурсов блокируется и создание, и удаление.
Чтобы вернуть возможность работы с ресурсами, восстановите Deployment `gatekeeper-controller-manager`.
{% endalert %}

{% alert level="warning" %}
Проверки на уровне контроллеров выполняются по **исходному шаблону пода** (`spec.template`),
а не по манифесту, полученному после всех мутаций.
Проверка на уровне подов, напротив, выполняется после всех мутирующих действий — например,
после того как LimitRange добавит `resources` по умолчанию, а мутирующий вебхук — `securityContext`.
Учитывайте это при использовании политик безопасности.
{% endalert %}

### Семантика labelSelector для контроллеров

При формировании политик, которые применяются на основе лейблов объектов (`match.labelSelector`),
учитывайте, что отбор происходит по `metadata.labels` проверяемого объекта.
Для подов это лейблы пода.
Для контроллеров (Deployment, StatefulSet и других) это **верхнеуровневые** `metadata.labels` контроллера,
а не `metadata.labels` шаблона пода (`spec.template.metadata.labels`).

Поэтому селектор, который отбирает поды, создаваемые контроллером, сам по себе не приводит к проверке
на уровне контроллера, а исключающий селектор (`NotIn`/`DoesNotExist`), исключающий такие поды из политики,
может не сработать на уровне контроллера.
В обоих случаях сам контроллер должен попадать под `labelSelector`.

То же относится к лейблу `security.deckhouse.io/skip-pss-check`, который исключает объект из ограничений Pod Security Standards.
Лейбл читается из `metadata.labels` проверяемого объекта, поэтому рабочая нагрузка, у которой он задан только в шаблоне пода
(`spec.template.metadata.labels`), исключается на уровне пода, но по-прежнему проверяется на уровне контроллера.
Чтобы исключить такую нагрузку полностью, задайте лейбл и в верхнеуровневых `metadata.labels` контроллера.
В отличие от SecurityPolicyException, разрешить этот лейбл из шаблона пода нельзя: исключение вычисляется
в `match.labelSelector` Gatekeeper, до выполнения кода политики.

### Разрешение лейблов SecurityPolicyException для контроллеров

Для контроллеров лейблы и аннотации SecurityPolicyException (SPE) берутся из **метаданных шаблона пода** (`spec.template.metadata`), а не из верхнеуровневых метаданных контроллера. Это гарантирует, что SPE применяется к подам, создаваемым контроллером, а не к самому объекту контроллера.

### Настройка проверки на уровне контроллеров

Проверку на уровне контроллеров настраивают четыре параметра с разной областью действия:

1. Параметр модуля [`controllerValidation`](configuration.html#parameters-podsecuritystandards-controllervalidation)
   включает или отключает проверку на уровне контроллеров для всех ограничений, которые читают спецификацию пода.
   Несмотря на то что параметр находится в секции `podSecurityStandards`, он действует на Pod Security Standards, OperationPolicy и SecurityPolicy.
1. Параметр модуля [`controllerValidationEnforcement`](configuration.html#parameters-podsecuritystandards-controllervalidationenforcement)
   задаёт, что происходит с контроллером, который нарушает запрещающую политику:
   при `Warn` (по умолчанию) он принимается с предупреждением, при `Policy` — запрещается.
   Параметр действует на Pod Security Standards и на все OperationPolicy и SecurityPolicy без `spec.controllerValidation.mode`.
1. Параметр модуля [`controllerValidationExcludedPolicies`](configuration.html#parameters-podsecuritystandards-controllervalidationexcludedpolicies)
   содержит правила Pod Security Standards, которые проверяются только на подах.
   Он действует только на Pod Security Standards.
1. Секция `spec.controllerValidation` ресурса [OperationPolicy](cr.html#operationpolicy-v1alpha1-spec-controllervalidation)
   или [SecurityPolicy](cr.html#securitypolicy-v1alpha1-spec-controllervalidation) настраивает эту политику.
   При `mode: Enforce` контроллеры проверяются с действием политики, при `mode: Warn` о нарушениях в них только предупреждают,
   при `mode: Disabled` проверяются только поды. `Enforce` и `Warn` включают проверку контроллеров, даже если параметр модуля её отключает,
   а политика без `mode` следует параметрам модуля.
   В `excludedPolicies` перечисляются правила политики, которые проверяются только на подах;
   список исключений модуля на политику не действует.

Исключите правило из проверки на уровне контроллеров, если под соответствует ему благодаря мутирующему вебхуку, а шаблон пода контроллера — нет.
Например, Vertical Pod Autoscaler задаёт поду `resources.requests`, а в шаблоне пода могут быть заданы только `limits`.
Контейнер, в котором `resources` не заданы совсем, и так проходит проверку на уровне контроллера
в [мягком режиме](#мягкий-режим-для-полей-которые-могут-быть-заданы-до-создания-пода),
поэтому исключение нужно, когда `resources` заданы частично.
Следующая OperationPolicy проверяет реестр образов на контроллерах, а запросы ресурсов — только на подах:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: OperationPolicy
metadata:
  name: requests-set-by-vpa
spec:
  enforcementAction: Deny
  controllerValidation:
    mode: Enforce
    excludedPolicies:
      - requiredResources
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          operation-policy.deckhouse.io/enabled: "true"
  policies:
    allowedRepos:
      - registry.example.com
    requiredResources:
      requests:
        - cpu
        - memory
```

Deployment, в шаблоне пода которого заданы `limits`, но не заданы `requests`, будет принят, а его поды будут отклонены, если Vertical Pod Autoscaler не задаст запросы.

Правила Pod Security Standards исключаются в настройках модуля.
Они называются так же, как параметры объектов SecurityPolicy, в виде которых опубликованы стандарты,
см. раздел [«Правила стандартов»](#правила-стандартов).
В следующем примере правила `allowPrivilegeEscalation` и `requiredDropCapabilities` проверяются только на подах:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: admission-policy-engine
spec:
  version: 1
  settings:
    podSecurityStandards:
      controllerValidationExcludedPolicies:
        - allowPrivilegeEscalation
        - requiredDropCapabilities
```

Некоторые правила проверяются одним ограничением и исключаются вместе: если указано любое правило группы, на подах проверяется вся группа.
Группы правил:

- `allowHostPID` и `allowHostIPC`;
- `allowHostNetwork` и `allowedHostPorts`;
- `fsGroup`, `runAsUser`, `runAsGroup` и `supplementalGroups`;
- `allowedUnsafeSysctls` и `forbiddenSysctls`.

Правила `allowedCapabilities` и `requiredDropCapabilities` исключаются по отдельности.

Если правило проверяется только на подах:

- контроллеры (Deployment, StatefulSet, DaemonSet, Job, CronJob, ReplicationController) не проверяются по нему при создании и обновлении;
- лейблы SecurityPolicyException на `spec.template.metadata.labels` контроллеров для него не разрешаются, так как оно не применяется к контроллерам;
- поды по-прежнему проверяются по нему при запуске.

### Мягкий режим для полей, которые могут быть заданы до создания пода

Некоторые ограничения используют **мягкий режим** (lenient mode) для полей, которые могут отсутствовать в шаблоне пода контроллера, но присутствовать в поде, создаваемом этим контроллером. Это не даёт отклонить нагрузку, поды которой в итоге соответствуют политике.

Следующие ограничения пропускают нарушения для отсутствующих полей при проверке контроллеров, но по-прежнему блокируют явно заданные недопустимые значения. Решение принимается для каждого контейнера отдельно: контейнер без поля обрабатывается мягко, даже если другой контейнер того же шаблона пода это поле задал.

| Ограничение | Пропускаемые поля (при отсутствии в контроллере) | Что может задать поле                                      |
|---|---|------------------------------------------------------------|
| `D8RequiredResources` | `container.resources` (когда не заданы ни `limits`, ни `requests`) | LimitRange                                                 |
| `D8AllowedUsers` | `runAsUser`, `runAsGroup`, `fsGroup`, `supplementalGroups` (с правилами `MustRunAs` / `MustRunAsNonRoot`) | Мутирующий вебхук, в том числе мутатор `Assign` Gatekeeper |

Например, Deployment без `resources` в шаблоне пода **не будет** отклонён ограничением `D8RequiredResources`, так как LimitRange в неймспейсе может добавить `requests` и `limits` по умолчанию. Однако если `resources` заданы частично (например, только `limits.memory`, но не `limits.cpu`), нарушение всё равно будет зафиксировано.

`D8RequiredResources` опирается на встроенный компонент Kubernetes, `D8AllowedUsers` — нет: в кластере без подходящего мутатора проверка этих полей на уровне контроллера не выполняется, и нарушение проявится при создании пода. Pod Security Admission в перечень источников не входит: этот компонент только проверяет объект и никогда его не изменяет.

`D8AllowedSeccompProfiles` в мягком режиме не нуждается: контейнер, для которого профиль не задан ни в одном источнике, разрешён и в поде, поэтому контроллеры и поды здесь ведут себя одинаково.

### Ограничения с неполной проверкой на уровне контроллеров

Некоторые проверки опираются на данные, которые появляются только после мутаций, поэтому не могут отклонить контроллер, хотя отклоняют его поды.

`D8AutomountServiceAccountToken` определяет монтирование токена либо по `spec.automountServiceAccountToken: true`, либо по контейнеру, который монтирует `/var/run/secrets/kubernetes.io/serviceaccount`. Плагин ServiceAccount admission добавляет это монтирование в под, а не в шаблон пода контроллера. Поэтому нагрузка, в которой `automountServiceAccountToken` просто не задан, проходит проверку на уровне контроллера, а отказ появляется при создании подов. Чтобы получить отказ на уровне контроллера, задайте в шаблоне пода `automountServiceAccountToken: false` явно.

## Валидация подов при изменении политики или добавлении новой

Для всех трех категорий политик (Pod Security Standards, операционные и политики безопасности) не предусмотрено автоматическое пересоздание существующих подов при изменении действующих или добавлении новых политик. Поды, существовавшие до момента внесения изменений в используемую политику или до добавления новой, продолжат работать до перезапуска. А при перезапуске они будут валидироваться по новым правилам.

В модуле `admission-policy-engine` предусмотрены алерты для таких случаев.
Каждый алерт срабатывает отдельно для каждой политики и неймспейса, в которых есть объекты с нарушениями:

| Алерт | О чём сообщает |
|---|---|
| `PodSecurityStandardsViolation` | Работающие поды, которые нарушают Pod Security Standard |
| `OperationPolicyViolation` | Существующие объекты, которые нарушают OperationPolicy |
| `SecurityPolicyViolation` | Существующие объекты, которые нарушают SecurityPolicy |
| `ControllerPolicyViolation` | Контроллеры, создающие поды, которые нарушают политику с `enforcementAction: Deny` и о которых на переходный период только предупреждают, см. [Проверка на уровне контроллеров](#проверка-на-уровне-контроллеров) |

Лейбл `policy` алерта содержит имя политики:
имя OperationPolicy или SecurityPolicy либо `d8-pod-security-baseline` или `d8-pod-security-restricted` для Pod Security Standards.
Лейбл `namespace` содержит неймспейс объектов с нарушениями.

Алерт — это ресурс ObservabilityAlert в неймспейсе объектов с нарушениями, поэтому его видят пользователи этого неймспейса.
Алерт о кластерных объектах — это ресурс ClusterObservabilityAlert.

Чтобы получить список алертов в неймспейсе, выполните следующую команду, где `<NAMESPACE>` — неймспейс:

```bash
d8 k -n <NAMESPACE> get observabilityalerts
```

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
NAME                                                        SEVERITY   STATUS   DURATION   SUMMARY                                                                                     AGE
PodSecurityStandardsViolation-07243a74fddd8234-1792744609   3          Firing   1h         Pods in the `app` namespace violate the `d8-pod-security-baseline` Pod Security Standard.   1h
SecurityPolicyViolation-8f21cd962274a252-1792744609         3          Firing   1h         Objects in the `app` namespace violate the `restricted-app` SecurityPolicy.                 1h
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

Чтобы получить список алертов во всех неймспейсах и алертов о кластерных объектах, выполните следующие команды:

```bash
d8 k get observabilityalerts -A
d8 k get clusterobservabilityalerts
```

Чтобы посмотреть алерт, выполните следующую команду:

```bash
d8 k -n <NAMESPACE> get observabilityalert <ALERT_NAME> -o yaml
```

Описание алерта содержит запрос Prometheus, который выводит объекты с нарушениями и нарушенные правила.

## Pod Security Standards

[Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) (`PSS`) — это официальный стандарт Kubernetes, который определяет три уровня безопасности для подов, ограничивая их привилегии. Ограничение происходит с помощью запрета установки определенных параметров в манифесте пода.

Используется многослойная структура — каждый более высокий уровень защиты использует все правила предыдущего уровня и добавляет свои.

В `PSS` регламентированы следующие уровни защиты (политики):

- `Privileged` — неограничивающая политика с максимально широким уровнем разрешений (отсутствие ограничений).
- `Baseline` — минимально ограничивающая политика, которая предотвращает наиболее известные и популярные способы повышения привилегий. Позволяет использовать стандартную (минимально заданную) конфигурацию пода.
- `Restricted` — политика со значительными ограничениями. Предъявляет самые жёсткие требования к подам.

{% alert level="info" %}
В Deckhouse Platform эти политики реализуются средствами Gatekeeper и контролируются admission-контроллерами модуля `admission-policy-engine`, а не контролером [Pod Security Admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/) от Kubernetes. Из Kubernetes взяты только описания политик.
{% endalert %}

Подробнее про каждый набор политик и их ограничения можно прочитать в [документации Kubernetes](https://kubernetes.io/docs/concepts/security/pod-security-standards/#profile-details).

Политика PSS для неймспейса включается через добавление на него специального лейбла `security.deckhouse.io/pod-policy=<POLICY_NAME>`.
Политику по умолчанию можно переопределить глобально ([в настройках модуля](configuration.html#parameters-podsecuritystandards-defaultpolicy)).

{% alert level="info" %}
В неймспейсах с именами `d8-*` и `kube-*` стандарт `restricted` применяется в режиме `warn`
независимо от политики по умолчанию и от лейблов неймспейса.
Нарушение стандарта фиксируется в отчётах безопасности и отображается в веб-интерфейсе Deckhouse Platform, нагрузка при этом запускается.
Если владеющий неймспейсом модуль включил для него проверку политик, стандарт также применяется в этом неймспейсе
с режимом, настроенным для этого неймспейса.
Лейбл `security.deckhouse.io/pod-policy-action` не может ослабить режим в этих неймспейсах ниже `warn`:
при значении `dryrun` пользователь всё равно получает предупреждения.
В таком неймспейсе о каждом нарушении сообщают обе проверки, поэтому в ответе API-сервера и в аудите оно встречается дважды.
Дашборд модуля учитывает такое нарушение один раз.
{% endalert %}

{% alert level="info" %}
При включении [модуля `multitenancy-manager`](/modules/multitenancy-manager/) он создаёт свои объекты OperationPolicy (например, в неймспейсе `default`). На них не влияют [настройки `podSecurityStandards`](configuration.html#parameters-podsecuritystandards).
{% endalert %}

Пример установки политики `Restricted` для всех подов в неймспейсе `my-namespace`:

```bash
d8 k label ns my-namespace security.deckhouse.io/pod-policy=restricted
```

Дополнительно возможна настройка режима работы политики.
Поддерживаются следующие режимы:

- `deny` — запретить запуск подов, не удовлетворяющих политике;
- `warn` — запускать поды, не удовлетворяющие политике, но выдавать предупреждение.
- `dryrun` — запускать поды не удовлетворяющие политике, не выдавать предупреждение пользователю, но фиксировать нарушения в отчетах безопасности;

Настройка режима работы политики производится путем установки лейбла `security.deckhouse.io/pod-policy-action=<POLICY_ACTION>` на соответствующем неймспейсе.
Чтобы задать режим работы политик глобально, используйте параметр [`enforcementaction`](configuration.html#parameters-podsecuritystandards-enforcementaction).

Пример установки `warn` режима политик PSS для всех подов в неймспейсе `my-namespace`:

```bash
d8 k label ns my-namespace security.deckhouse.io/pod-policy-action=warn
```

### Правила стандартов

Модуль публикует правила стандартов в виде двух объектов SecurityPolicy:
`d8-pod-security-baseline` и `d8-pod-security-restricted`.
Правила записаны в формате параметров [SecurityPolicy](cr.html#securitypolicy) — тех же, что используются в ваших политиках безопасности.
Объект `d8-pod-security-restricted` содержит все правила, по которым проверяется неймспейс с уровнем `Restricted`, включая правила `Baseline`.
Два значения в правилах не описаны в SecurityPolicy:
`""` и `undefined` в `seccompProfiles.allowedProfiles` разрешают контейнер без профиля seccomp,
а `type: ""` в `seLinux` разрешает контейнер без типа SELinux.

Чтобы посмотреть правила стандарта `Restricted`, выполните команду:

```bash
d8 k get securitypolicy d8-pod-security-restricted -o yaml
```

В секции `spec.policies` перечислены правила, а в секции `spec.controllerValidation` — настройки проверки на уровне контроллеров
из конфигурации модуля.

Объекты описывают стандарты и сами по себе ничего не проверяют.
Модуль применяет стандарты к неймспейсам по лейблу `security.deckhouse.io/pod-policy` и настройкам модуля,
а в `spec.match` объектов показаны прикладные неймспейсы, к которым стандарт применяется в режиме по умолчанию.
Объекты управляются модулем, изменить их нельзя.
Настраивайте стандарты в [параметрах модуля](configuration.html#parameters-podsecuritystandards).
Если ваша SecurityPolicy уже называется `d8-pod-security-baseline` или `d8-pod-security-restricted`,
модуль сохраняет вашу политику и не публикует этот стандарт. Стандарт при этом продолжает применяться.
Чтобы увидеть его правила, переименуйте свою политику.

## Операционные политики

Операционные политики — это правила, направленные на достижение лучших практик безопасности приложений, но **не относящиеся** напрямую к валидации классических параметров, связанных с безопасностью (например, список допустимых префиксов для образов контейнеров, политика скачивания образов, список необходимых проб для контейнеров и т.д.).

Операционные политики описываются с помощью кастомного ресурса [`OperationPolicy`](/modules/admission-policy-engine/cr.html#operationpolicy).
В нём каждый параметр отвечает за отдельную проверку, применяемую к ресурсам.
Использование кастомного ресурса OperationPolicy позволяет создавать дополнительные требования к создаваемым ресурсам (высокоуровневые декларативные операционные политики) без явной работы с Gatekeeper.

Рекомендуется устанавливать следующий минимальный набор операционных политик:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: OperationPolicy
metadata:
  name: common
spec:
  enforcementAction: Deny
  policies:
    allowedRepos:
      - myrepo.example.com
      - registry.deckhouse.ru
    requiredResources:
      limits:
        - memory
      requests:
        - cpu
        - memory
    disallowedImageTags:
      - latest
    requiredProbes:
      - livenessProbe
      - readinessProbe
    maxRevisionHistoryLimit: 3
    imagePullPolicy: Always
    priorityClassNames:
    - production-high
    - production-low
    checkHostNetworkDNSPolicy: true
    checkContainerDuplicates: true
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          custom-operation-policy/enabled: "true"
```

Применение политики реализовано через настройки, расположенные в параметре `spec.match`.

При указании:

```yaml
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          custom-operation-policy/enabled: "true"
```

Для применения приведённой политики достаточно добавить лейбл `custom-operation-policy/enabled: "true"` на желаемый неймспейс.  
В отличие от `PSS`, название лейбла может быть любым. Требуется лишь совпадение лейбла в селекторе политик и соответствующего неймспейса.

Более подробную информацию об использовании селекторов вы можете прочитать в [описании настройки селекторов](/modules/admission-policy-engine/faq.html#как-настроить-селекторы-политик).

Для политики также возможно указание применяемого действия.
Для этого используется параметр `spec.enforcementAction`.
Поддерживаются следующие режимы:

- `Deny` — запретить запуск подов не удовлетворяющих политике;
- `Warn` — запускать поды не удовлетворяющие политике, но выдавать предупреждение.
- `Dryrun` — запускать поды не удовлетворяющие политике, не выдавать предупреждение пользователю, но фиксировать нарушения в отчетах безопасности;

Основываясь на этом примере, вы можете создать собственную политику с необходимыми настройками.

## Политики безопасности

Политики безопасности — это правила, направленные на достижение лучших практик безопасности приложений с помощью валидации значений параметров, связанных с безопасностью (например, доступ контейнеров к IPC- или PID-пространству имен хоста, список привилегий для контейнеров и т.д.).

Политики безопасности описываются с помощью кастомного ресурса [`SecurityPolicy`](/modules/admission-policy-engine/cr.html#securitypolicy).
В нём каждый параметр отвечает за отдельную проверку применяемую к ресурсам.
С помощью этого ресурса возможно сконструировать политику безопасности аналогичную политике PSS любого уровня.
Использование кастомного ресурса SecurityPolicy позволяет создавать дополнительные требования к создаваемым ресурсам (высокоуровневые декларативные политики безопасности) без явной работы с Gatekeeper.

Пример политики безопасности:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: mypolicy
spec:
  enforcementAction: Deny
  policies:
    allowHostIPC: true
    allowHostNetwork: true
    allowHostPID: false
    allowPrivileged: false
    allowPrivilegeEscalation: false
    allowedFlexVolumes:
    - driver: vmware
    allowedHostPorts:
    - max: 4000
      min: 2000
    allowedProcMount: Unmasked
    allowedAppArmor:
    - unconfined
    allowedUnsafeSysctls:
    - kernel.*
    allowedVolumes:
    - hostPath
    - projected
    fsGroup:
      ranges:
      - max: 200
        min: 100
      rule: MustRunAs
    readOnlyRootFilesystem: true
    requiredDropCapabilities:
    - ALL
    runAsGroup:
      ranges:
      - max: 500
        min: 300
      rule: RunAsAny
    runAsUser:
      ranges:
      - max: 200
        min: 100
      rule: MustRunAs
    seccompProfiles:
      allowedLocalhostFiles:
      - my_profile.json
      allowedProfiles:
      - Localhost
    supplementalGroups:
      ranges:
      - max: 133
        min: 129
      rule: MustRunAs
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          security-policy: mypolicy
```

{% alert level="warning" %}
Параметры `allowPrivilegeEscalation` и `allowPrivileged` по умолчанию имеют значение `false` — даже если не указаны явно. Это означает, что контейнеры не смогут запускаться в привилегированном режиме или повышать привилегии. Чтобы разрешить такое поведение, задайте параметр в `true`.
{% endalert %}

Применение политики реализовано через настройки расположенные в параметре `spec.match`.

При указании:

```yaml
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          security-policy: mypolicy
```

Для применения приведённой политики достаточно добавить лейбл `security-policy: mypolicy` на желаемый неймспейс.  
В отличие от `PSS`, название лейбла может быть любым. Требуется лишь совпадение лейбла в селекторе политик и соответствующего неймспейса.

Более подробную информацию о использовании селекторов вы можете прочитать в [описании настройки селекторов](/modules/admission-policy-engine/faq.html#как-настроить-селекторы-политик).

Для политики также возможно указание применяемого действия.
Для этого используется параметр `spec.enforcementAction`.
Поддерживаются следующие режимы:

- `Deny` — запретить запуск подов, не удовлетворяющих политике;
- `Warn` — запускать поды, не удовлетворяющие политике, но выдавать предупреждение.
- `Dryrun` — запускать поды не удовлетворяющие политике, не выдавать предупреждение пользователю, но фиксировать нарушения в отчетах безопасности;

## Исключения из политик безопасности

[SecurityPolicyException](cr.html#securitypolicyexception) — это ресурс, позволяющий создавать точечные исключения из проверок политик безопасности для отдельных подов и контейнеров. Он позволяет не отключать проверки для всего неймспейса, а описывать только необходимые исключения из конкретного правила для пода или контейнера.

### Добавление исключений

Чтобы добавить исключения для пода или контейнера, выполните следующее:

1. Создайте объект [SecurityPolicyException](cr.html#securitypolicyexception), описав необходимые исключения.

   Рекомендуется документировать причину каждого исключения в поле `metadata` соответствующего правила (например, `metadata.description`). Это упрощает последующие аудит и сопровождение.

2. В шаблоне пода (обычно через поле `spec.template.metadata.labels` ресурса Deployment, StatefulSet или DaemonSet) укажите один следующих лейблов со ссылкой на исключение:
   - `security.deckhouse.io/security-policy-exception: <exception-name>` — исключение для всего пода;
   - `security.deckhouse.io/security-policy-exception.container.<container-name>: <exception-name>` — исключение для конкретного контейнера.

Приоритет выбора исключения для контейнера:

1. Сначала проверяется лейбл `security.deckhouse.io/security-policy-exception.container.<container-name>`.
1. Если лейбл для конкретного контейнера отсутствует, используется исключение из `security.deckhouse.io/security-policy-exception`.

{% alert level="warning" %}
Если для контейнера задан отдельный лейбл, но он указывает на несуществующий или некорректный объект SecurityPolicyException, он всё равно имеет приоритет над общим лейблом и может привести к запрету размещения пода.
{% endalert %}

### Пример конфигурации

Для примера рассмотрим под, которому требуется:

- разрешение на использование настройки [`hostNetwork`](/products/kubernetes-platform/documentation/v1/user/security/pod-settings.html#hostnetwork) всему поду;
- разрешение на использование настройки [`privileged`](/products/kubernetes-platform/documentation/v1/user/security/pod-settings.html#privileged) только для контейнера `sample-init`.

Без использования ресурса SecurityPolicyException для разрешения этих параметров потребовалось бы создать пользовательскую политику безопасности, допускающую их использование для всех подов в кластере.

При использовании SecurityPolicyException достаточно создать следующие ресурсы:

- Исключение для разрешения параметра `hostNetwork`:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: SecurityPolicyException
  metadata:
    name: allow-hostnetwork-pod
  spec:
    network:
      hostNetwork:
        allowedValue: true
        metadata:
          description: >-
            Pod requires host network mode for node-level network diagnostics.
  ```

- Исключение для разрешения параметра `privileged` в контейнере `sample-init`:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: SecurityPolicyException
  metadata:
    name: allow-privileged-init-container
  spec:
    securityContext:
      privileged:
        allowedValue: true
        metadata:
          description: >-
            Container init requires privileged mode to access host-level networking features.
  ```

После этого необходимо добавить соответствующие лейблы в шаблоне пода:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: example
spec:
  template:
    metadata:
      labels:
        # Общее исключение, применяемое ко всему поду.
        security.deckhouse.io/security-policy-exception: allow-hostnetwork-pod
        # Исключение для контейнера sample-init.
        security.deckhouse.io/security-policy-exception.container.sample-init: allow-privileged-init-container
    spec:
      hostNetwork: true         
    ...
    containers:
      - name: sample-init 
        securityContext:
          privileged: true 
```

## Изменение ресурсов Kubernetes

Модуль позволяет использовать [кастомные ресурсы Gatekeeper](gatekeeper-cr.html) для модификации объектов в кластере, такие как:

- [AssignMetadata](gatekeeper-cr.html#assignmetadata) — для изменения секции `metadata` в ресурсе;
- [Assign](gatekeeper-cr.html#assign) — для изменения других полей, кроме `metadata`;
- [ModifySet](gatekeeper-cr.html#modifyset) — для добавления или удаления значений из списка, например аргументов для запуска контейнера.
- [AssignImage](gatekeeper-cr.html#assignimage) — для изменения параметра `image` ресурса.

Подробнее про доступные варианты можно прочитать в документации [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/).

## Доступность компонентов модуля

Модуль находится на критическом пути кластера: пока его validating-вебхук недоступен, API-сервер отклоняет запросы, которые вебхук перехватывает.

### Почему validating-вебхук критичен для кластера

Deployment `gatekeeper-controller-manager` обслуживает ValidatingWebhookConfiguration с именем `d8-admission-policy-engine-config`. Его основной вебхук работает с `failurePolicy: Fail`: запрос, который API-сервер не смог доставить вебхуку, отклоняется, а не пропускается.

Пока вебхук недоступен, ни один объект не обходит политики, но и создать или изменить объекты, которые он перехватывает, в это время нельзя.

Пока ни одна реплика `gatekeeper-controller-manager` не доступна, в неймспейсах, которые охватывает вебхук, перестаёт работать следующее:

- операции с объектами из раздела [«Проверка на уровне контроллеров»](#проверка-на-уровне-контроллеров), включая `d8 k exec` и `d8 k attach`, а также `d8 k debug`;
- создание, изменение и удаление Role, RoleBinding и ограничений Gatekeeper.

Mutating-вебхук настроен иначе: его `failurePolicy` — `Ignore`, и недоступность Deployment `gatekeeper-controller-manager` означает лишь то, что мутации не применяются.

Отключить модуль и разблокировать кластер можно, но вместе с модулем пропадает и контроль: политики больше не применяются, и объект, который политика запрещала, создаётся беспрепятственно. Порядок действий при недоступном вебхуке описан в [FAQ](faq.html#что-делать-если-validating-вебхук-недоступен).

### Какие объекты не проверяются

Исключения определяют, какие объекты можно создавать и изменять, пока вебхук недоступен.

Вебхуки модуля не проверяют:

- объекты в неймспейсах с лейблом `heritage: deckhouse` — основной вебхук исключает их через `namespaceSelector`;
- объекты с лейблом `gatekeeper.sh/operation: webhook`, который Gatekeeper проставляет собственным подам, — вебхуки исключают их через `objectSelector`;
- запросы от сервисных аккаунтов неймспейса `d8-virtualization`.

Неймспейсы, у которых есть ещё и лейбл `security.deckhouse.io/enable-security-policy-check`, в том числе `d8-admission-policy-engine`, проверяет отдельный вебхук.

Исключение по лейблу `gatekeeper.sh/operation: webhook` необходимо в неймспейсе модуля. Без этого исключения вебхук не смог бы восстановиться после собственного отказа. Когда исчезает последняя реплика, API-сервер отклонил бы создание замещающего пода, поскольку проверить его некому.

На `d8 k exec` и `d8 k attach` исключение по лейблу не распространяется. У объекта такого запроса не может быть лейблов, и API-сервер не вызывает для него вебхук с `objectSelector`. Эти запросы перехватывает отдельный вебхук без `objectSelector`, который исключает те же неймспейсы, что и основной. В неймспейсах `d8-*` и `kube-*` они до Gatekeeper не доходят.
