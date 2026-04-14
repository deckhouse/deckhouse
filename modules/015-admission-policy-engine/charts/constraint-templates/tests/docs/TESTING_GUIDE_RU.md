# Руководство по тестированию Constraint Templates (RU)

Руководство охватывает всё, что нужно для написания, запуска и поддержки тестов Gatekeeper ConstraintTemplates в Deckhouse. Написано для новичков с **нулевым контекстом**.

> **Схемы валидации** для каждого YAML-файла, описанного ниже, находятся в [`../openapi/`](../openapi/). Используйте их как авторитетный справочник по допустимым полям и значениям.

---

## Содержание

1. [Словарь терминов](#1-словарь-терминов)
2. [Структура каталогов](#2-структура-каталогов)
3. [Файлы для каждого constraint](#3-файлы-для-каждого-constraint)
4. [test_fields.yaml — модель полей/сценариев](#4-test_fieldsyaml--модель-полейсценариев)
5. [test-matrix.yaml — тест-кейсы](#5-test-matrixyaml--тест-кейсы)
6. [test_profile.yaml — контракт suite/качества](#6-test_profileyaml--контракт-suiteкачества)
7. [Модель сценариев](#7-модель-сценариев)
8. [Расчёт покрытия](#8-расчёт-покрытия)
9. [Пошагово: добавление тестов для нового constraint](#9-пошагово-добавление-тестов-для-нового-constraint)
10. [Пошагово: добавление тестов к существующему constraint](#10-пошагово-добавление-тестов-к-существующему-constraint)
11. [Полезные команды](#11-полезные-команды)
12. [Известные ограничения](#12-известные-ограничения)
13. [Устранение неполадок](#13-устранение-неполадок)
14. [Definition of Done](#14-definition-of-done)

---

## 1. Словарь терминов

| Термин                  | Значение                                                                                                                                                                             |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Constraint**          | Политика Gatekeeper, которая валидирует объекты Kubernetes (например, Pod). У каждого constraint своя директория тестов.                                                             |
| **ConstraintTemplate**  | Шаблон на Rego, определяющий логику политики. Находится в `charts/constraint-templates/templates/`.                                                                                  |
| **SPE**                 | SecurityPolicyException — CRD Deckhouse, задающий исключения для constraint. Поля SPE описаны в [`security-policy-exception.yaml`](../../../../crds/security-policy-exception.yaml). |
| **Track (трек)**        | Группа тестов: *Functional*, *SPE Pod* или *SPE Container*.                                                                                                                          |
| **Scenario (сценарий)** | Конкретный ракурс тестирования поля (positive, negative, absent и т.д.).                                                                                                             |
| **Block (блок)**        | Именованная секция в сгенерированном test suite (`rendered/test_suite.yaml`), группирующая кейсы с общей парой template+constraint.                                                  |
| **Gator**               | CLI-инструмент OPA Gatekeeper для офлайн-проверки тестов constraint.                                                                                                                 |
| **constraint_testgen**  | Go-инструмент, преобразующий `test-matrix.yaml` в сгенерированные тестовые артефакты.                                                                                                |

---

## 2. Структура каталогов

Вся тестовая инфраструктура находится в:

```shell
charts/constraint-templates/tests/
├── docs/                  # Документация (этот файл)
├── openapi/               # JSON Schema файлы для валидации
│   ├── constraint-test-fields.schema.yaml
│   ├── constraint-test-matrix.schema.yaml
│   └── constraint-test-profile.schema.yaml
├── tools/
│   └── constraint_testgen/   # Go-инструмент: generate, verify, coverage
├── test_cases/
│   ├── run_all_tests.sh      # Главный скрипт запуска (OPA + gator + coverage)
│   └── constraints/
│       ├── security/          # Constraint-ы безопасности
│       │   ├── allow-host-network/
│       │   ├── allow-privilege-escalation/
│       │   ├── allow-privileged/
│       │   └── ...
│       └── operation/         # Операционные constraint-ы
│           ├── allowed-repos/
│           ├── container-resources/
│           └── ...
├── README.md
└── AGENTS.md              # Промпт для AI-агента
```

Группы constraint-ов: **`security`** и **`operation`**.

---

## 3. Файлы для каждого constraint

Каждая директория constraint (например, `test_cases/constraints/security/allow-host-network/`) содержит:

| Файл                                | Назначение                                                                                                | Ручной? |
| ----------------------------------- | --------------------------------------------------------------------------------------------------------- | :-----: |
| `test_fields.yaml`                  | Описывает, какие поля проверяет политика и какие сценарии обязательны. Источник истины для покрытия.      |    ✅    |
| `test-matrix.yaml`                  | Описывает тест-кейсы с аннотациями `fields`, связывающими кейс с парами поле+сценарий.                    |    ✅    |
| `test_profile.yaml`                 | Контракт suite/качества: обязательные имена тестовых блоков, опциональные quality gates.                  |    ✅    |
| `constraints/`                      | YAML-манифесты constraint, на которые ссылается матрица (например, `pss_baseline.yaml`, `policy_1.yaml`). |    ✅    |
| `rendered/`                         | **Сгенерированные артефакты — не редактировать вручную.**                                                 |    ❌    |
| `rendered/test_suite.yaml`          | Плоский план тестов для gator.                                                                            |    ❌    |
| `rendered/test_samples/`            | Сгенерированные YAML-примеры Pod/объектов для каждого кейса.                                              |    ❌    |
| `rendered/constraint-template.yaml` | Отрендеренный ConstraintTemplate из Helm chart.                                                           |    ❌    |
| `rendered/constraints/`             | Отрендеренные копии constraint для gator.                                                                 |    ❌    |

### Соглашения по именованию файлов constraint

- `pss_baseline.yaml` — constraint для baseline-совместимых тестов
- `pss_restricted.yaml` — constraint для restricted-совместимых тестов
- `policy_<n>.yaml` — дополнительные сценарно-специфичные constraint-ы (нумерация с 1)

---

## 4. test_fields.yaml — модель полей/сценариев

### Назначение

Файл описывает, **что именно проверяет политика** и **сколько тестовых сценариев требуется для каждого поля**. Это **источник истины** для расчёта сценарного покрытия.

### Схема

> Полная схема: [`../openapi/constraint-test-fields.schema.yaml`](../openapi/constraint-test-fields.schema.yaml)

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestFields
metadata:
  name: <имя-constraint>          # Должно совпадать с именем директории
spec:
  objectKind: Pod                   # Kubernetes kind под тестом
  objectFields:                     # Поля, которые Rego читает/сравнивает
    - path: spec.hostNetwork
      level: pod                    # pod | container | initContainer
      description: "Использует ли pod host network namespace"
      requiredScenarios:            # Опустите для использования значений по умолчанию
        - positive
        - negative
        - absent
  speFields:                        # Опционально: поля SPE, переопределяющие constraint
    - path: spec.network.hostNetwork.allowedValue
      level: pod                    # pod | container
      description: "SPE-исключение для hostNetwork"
      requiredScenarios:
        - speMatch
        - speMismatch
        - speAbsent
  applicableTracks:
    functional: true                # Есть обычные (не SPE) кейсы
    spePod: true                    # SPE на уровне pod
    speContainer: false             # SPE на уровне контейнера
```

### Ключевые правила

1. **`metadata.name`** должен совпадать с именем директории constraint.
2. **`objectKind`** — основной Kubernetes kind, который валидируется (обычно `Pod`).
3. **`objectFields`** — все поля объекта, которые Rego читает или сравнивает.
4. **`speFields`** — все поля SPE, которые Rego читает. Всегда сверяйте пути с CRD SPE.
5. **`level`** определяет сценарии по умолчанию:
   - `pod` → поля уровня Pod (`spec.*`)
   - `container` → поля внутри `spec.containers[].*`
   - `initContainer` → поля внутри `spec.initContainers[].*`
6. **`requiredScenarios`** можно опустить для использования значений по умолчанию (см. [Модель сценариев](#7-модель-сценариев)).
7. **`applicableTracks`** — хотя бы один трек должен быть `true`.

### Реальный пример (allow-host-network)

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestFields
metadata:
  name: allow-host-network
spec:
  objectKind: Pod
  objectFields:
    - path: spec.hostNetwork
      level: pod
      description: Использует ли pod host network namespace
      requiredScenarios: [positive, negative, absent]
    - path: spec.containers[].ports[].hostPort
      level: container
      description: Host port для контейнера
      requiredScenarios: [positive, negative, absent, multiContainer, initContainer, ephemeralContainer]
    - path: spec.containers[].ports[].protocol
      level: container
      description: Протокол host port
      requiredScenarios: [positive, negative, absent, multiContainer, initContainer, ephemeralContainer]
  speFields:
    - path: spec.network.hostNetwork.allowedValue
      level: pod
      description: SPE-исключение для hostNetwork
      requiredScenarios: [speMatch, speMismatch, speAbsent]
    - path: spec.network.hostPorts[].port
      level: pod
      description: Разрешённый host port в SPE
      requiredScenarios: [speMatch, speMismatch, speAbsent]
    - path: spec.network.hostPorts[].protocol
      level: pod
      description: Разрешённый протокол host port в SPE
      requiredScenarios: [speMatch, speMismatch, speAbsent]
  applicableTracks:
    functional: true
    spePod: true
    speContainer: false
```

---

## 5. test-matrix.yaml — тест-кейсы

### Назначение

Определяет **тест-кейсы**, организованные в **блоки**. Каждый кейс декларирует, какие пары поле+сценарий он покрывает через массив `fields`, что позволяет автоматически рассчитывать покрытие.

### Схема

> Полная схема: [`../openapi/constraint-test-matrix.schema.yaml`](../openapi/constraint-test-matrix.schema.yaml)

### Структура верхнего уровня

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestMatrix
metadata:
  name: <имя-constraint>
spec:
  suiteName: d8-<имя-constraint>        # Имя для сгенерированного Suite
  outputTestDirectory: rendered           # Директория вывода (всегда "rendered")
  defaultObjectBase: admissionPod         # Base по умолчанию для объектов кейсов
  defaultInventory:                       # Inventory, добавляемый к каждому кейсу
    - ref: ../../../_test-samples/ns.yaml
  bases:                                  # Переиспользуемые базовые документы
    admissionPod:
      document:
        apiVersion: v1
        kind: Pod
        metadata:
          namespace: testns
        spec:
          containers:
            - image: nginx
              name: nginx
    securityPolicyException:
      document:
        apiVersion: deckhouse.io/v1alpha1
        kind: SecurityPolicyException
        metadata:
          namespace: testns
  namedExceptions: {}                     # Переиспользуемые фрагменты исключений
  externalData:                           # Опционально: external data providers для gator
    providers: []
  blocks:                                 # Тестовые блоки (см. ниже)
    - name: ...
```

### Блоки

Каждый блок соответствует записи `tests[]` в сгенерированном `test_suite.yaml`:

```yaml
blocks:
  - name: pss-baseline-functional         # Человекочитаемое имя
    gatorBlock: pss-baseline-functional    # Имя блока для gator (переопределяет name)
    template: ../rendered/constraint-template.yaml
    constraint: constraints/pss_baseline.yaml
    cases:
      - name: allowed-no-hostnetwork
        violations: "no"                   # "yes" или "no"
        fields:                            # Аннотации покрытия
          - path: spec.hostNetwork
            scenario: absent
        object:                            # Тестируемый объект
          base: admissionPod
          merge:
            metadata:
              name: allowed-no-hostnetwork
            spec:
              containers:
                - image: nginx
                  name: nginx
                  ports:
                    - containerPort: 80
```

### Паттерн `object.merge`

Кейсы используют паттерн **base + merge**:
- `base` ссылается на ключ из `spec.bases`
- `merge` — deep-merge патч, применяемый поверх базового документа
- Словари рекурсивно мержатся; **массивы в `merge` заменяют** весь массив по этому пути
- Используйте `containerMerges` / `initContainerMerges` для точечных патчей контейнеров, когда base уже определяет этот контейнер

### Аннотации fields

Каждый кейс должен декларировать, какие пары поле+сценарий он покрывает:

```yaml
fields:
  - path: spec.hostNetwork        # Должен точно совпадать с путём из test_fields.yaml
    scenario: negative             # Должен быть валидным именем сценария
  - path: spec.containers[].ports[].hostPort
    scenario: multiContainer
```

**Правила:**
- Каждый `path` должен **побайтово совпадать** с `path` из `test_fields.yaml`
- Каждый `scenario` должен быть валидным именем сценария
- Для кейсов исключений используйте пути SPE-полей и SPE-сценарии
- Один кейс может покрывать несколько пар поле+сценарий
- Несколько кейсов могут покрывать одну и ту же пару (для покрытия достаточно одного)

### SPE-кейсы

Для кейсов SecurityPolicyException добавьте исключение как inventory:

```yaml
cases:
  - name: allowed-by-exception-hostnetwork
    violations: "no"
    fields:
      - path: spec.network.hostNetwork.allowedValue
        scenario: speMatch
    inventory:
      - base: securityPolicyException
        merge:
          metadata:
            name: allow-hostnetwork-true
          spec:
            network:
              hostNetwork:
                allowedValue: true
    object:
      base: admissionPod
      merge:
        metadata:
          labels:
            security.deckhouse.io/security-policy-exception: allow-hostnetwork-true
          name: allowed-by-exception-hostnetwork
        spec:
          hostNetwork: true
          containers:
            - image: nginx
              name: nginx
```

Ключевые моменты для SPE-кейсов:
- Pod должен иметь лейбл `security.deckhouse.io/security-policy-exception: <имя-исключения>`
- Имя исключения в лейбле должно совпадать с `metadata.name` SPE
- SPE inventory использует `base: securityPolicyException` с патчем `merge`

### Реальный пример (операционный constraint — allowed-repos)

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestMatrix
metadata:
  name: allowed-repos
spec:
  suiteName: d8-allowed-repos
  outputTestDirectory: rendered
  defaultObjectBase: admissionPod
  defaultInventory:
    - ref: ../../../_test-samples/ns.yaml
  bases:
    admissionPod:
      document:
        apiVersion: v1
        kind: Pod
        metadata:
          namespace: testns
        spec:
          containers:
            - image: nginx
              name: nginx
  blocks:
    - name: operation-policy
      gatorBlock: operation-policy
      template: ../../templates/operation/allowed-repos.yaml
      constraint: constraints/policy_1.yaml
      cases:
        - name: example-allowed
          violations: "no"
          fields:
            - path: spec.containers[].image
              scenario: positive
          object:
            base: admissionPod
            merge:
              metadata:
                name: allowed
              spec:
                containers:
                  - name: foo
                    image: my.repo/app:v1
        - name: example-disallowed
          violations: "yes"
          fields:
            - path: spec.containers[].image
              scenario: negative
          object:
            base: admissionPod
            merge:
              metadata:
                name: disallowed
              spec:
                containers:
                  - name: foo
                    image: gcr.io/app:v1
```

---

## 6. test_profile.yaml — контракт suite/качества

### Назначение

Per-constraint профиль верификации. Декларирует, какие тестовые блоки **обязаны** присутствовать в сгенерированном suite, и опциональные quality gates.

### Схема

> Полная схема: [`../openapi/constraint-test-profile.schema.yaml`](../openapi/constraint-test-profile.schema.yaml)

### Минимальный шаблон

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestProfile
metadata:
  name: <имя-constraint>
spec:
  testDirectory: <имя-constraint>
  suite:
    expectedTestBlockNames:
      - <имя-блока-1>
      - <имя-блока-2>
```

### Расширенный шаблон (опциональные quality gates)

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestProfile
metadata:
  name: <имя-constraint>
spec:
  testDirectory: <имя-constraint>
  suite:
    expectedTestBlockNames:
      - pss-baseline-functional
      - security-policy-functional
  coverage:
    minimumCasesPerBlock: 1
    requiredPatterns:
      functional:
        - "*negative*"
      securityPolicyExceptionPod:
        - "*spe*"
```

### Реальный пример (allow-host-network)

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestProfile
metadata:
  name: allow-host-network
spec:
  testDirectory: allow-host-network
  suite:
    expectedTestBlockNames:
      - pss-baseline-functional
      - security-policy-functional
      - security-policy-spe-pod
```

### Как сформировать test_profile.yaml

1. Установите `metadata.name` равным имени директории constraint.
2. Установите `spec.testDirectory` равным этому же имени.
3. Заполните `spec.suite.expectedTestBlockNames` всеми именами блоков из вашего `test-matrix.yaml` (значения `gatorBlock` или `name`).
4. При необходимости задайте `spec.coverage.*` для более строгих quality gates.

### Разделение ответственности

| Файл                | Зона ответственности                                                                        | НЕ определяет                                                     |
| ------------------- | ------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- |
| `test_fields.yaml`  | Модель полей/сценариев: объектные/SPE-поля, обязательные сценарии, применимые треки         | Имена блоков suite, минимум кейсов на блок, обязательные паттерны |
| `test_profile.yaml` | Контракт suite/качества: обязательные тестовые блоки, минимум кейсов, обязательные паттерны | Перечень полей и сценарии на уровне полей                         |

---

## 7. Модель сценариев

**Сценарий** — это конкретный ракурс тестирования поля. Обязательные сценарии определяют **минимальный набор ракурсов**, которые должны быть покрыты, чтобы поле считалось полноценно протестированным.

### Сценарии для полей объекта (Functional track)

| Сценарий             | Значение                                                    | Применяется к            |
| -------------------- | ----------------------------------------------------------- | ------------------------ |
| `positive`           | Поле задано допустимым значением → нет нарушения            | Все поля объекта         |
| `negative`           | Поле задано недопустимым значением → нарушение              | Все поля объекта         |
| `absent`             | Поле не задано → зависит от defaultBehavior                 | Все поля объекта         |
| `multiContainer`     | Несколько контейнеров, один нарушает → нарушение            | Только уровень container |
| `initContainer`      | Вариант проверки для initContainer                          | Только уровень container |
| `ephemeralContainer` | Вариант для ephemeralContainer (`spec.ephemeralContainers`) | Только уровень container |

### SPE-сценарии (Exception tracks)

| Сценарий               | Значение                                          | Применяется к               |
| ---------------------- | ------------------------------------------------- | --------------------------- |
| `speMatch`             | SPE совпадает с нарушением → исключение разрешает | Все поля SPE                |
| `speMismatch`          | SPE не совпадает → нарушение остаётся             | Все поля SPE                |
| `speAbsent`            | Нет SPE-метки на pod → нарушение остаётся         | Все поля SPE                |
| `speContainerSpecific` | SPE нацелен на конкретный контейнер               | Только SPE уровня container |

### Обязательные сценарии по умолчанию по уровню

**Поля объекта:**

| Уровень         | Сценарии по умолчанию                                                         | Кол-во |
| --------------- | ----------------------------------------------------------------------------- | ------ |
| `pod`           | positive, negative, absent                                                    | 3      |
| `container`     | positive, negative, absent, multiContainer, initContainer, ephemeralContainer | 6      |
| `initContainer` | positive, negative, absent                                                    | 3      |

**Поля SPE:**

| Уровень     | Сценарии по умолчанию                                  | Кол-во |
| ----------- | ------------------------------------------------------ | ------ |
| `pod`       | speMatch, speMismatch, speAbsent                       | 3      |
| `container` | speMatch, speMismatch, speAbsent, speContainerSpecific | 4      |

Если `requiredScenarios` не указан в `test_fields.yaml`, инструмент автоматически подставляет набор по умолчанию на основе `level`.

---

## 8. Расчёт покрытия

`constraint_testgen coverage` считает сценарное покрытие по `test_fields.yaml` + `test-matrix.yaml` и читает `test_profile.yaml` для профильных проверок.

### Формула

```shell
Покрытие поля     = покрытые сценарии / обязательные сценарии
Общее покрытие %  = сумма(покрытых сценариев) / сумма(обязательных сценариев) × 100
```

Сценарий считается «покрытым», если хотя бы один кейс в `test-matrix.yaml` указывает `fields` с этой парой path+scenario.

### Как достичь 100% покрытия

1. Для каждого поля в `test_fields.yaml` проверьте его `requiredScenarios` (или значения по умолчанию).
2. Для каждого обязательного сценария убедитесь, что хотя бы один кейс в `test-matrix.yaml` имеет соответствующую запись `fields` с точным `path` и `scenario`.
3. Запустите coverage для проверки — пропущенные сценарии будут перечислены как предупреждения.

### Пример вывода

```shell
Constraint              Fields  Scenarios  Covered  %     Status
allow-host-network      4+3     25         25       100%  OK
allow-privilege-escal.  1+1     9          6        67%   WARN
  missing: spec.containers[].securityContext.allowPrivilegeEscalation/multiContainer
  missing: spec.containers[].securityContext.allowPrivilegeEscalation/initContainer
```

---

## 9. Пошагово: добавление тестов для нового constraint

Используйте этот алгоритм при создании тестов для совершенно нового constraint с нуля.

### Шаг 1: Определите входы политики

- Прочитайте Rego-шаблон constraint и выпишите все поля объекта, которые он читает/сравнивает.
- Прочитайте параметры template и выпишите переключатели, влияющие на поведение политики.
- Прочитайте пути SPE в CRD [`security-policy-exception.yaml`](../../../../crds/security-policy-exception.yaml) и сопоставьте только те пути, которые реально используются в этом constraint.

### Шаг 2: Создайте test_fields.yaml

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestFields
metadata:
  name: <ваш-constraint>
spec:
  objectKind: Pod
  objectFields:
    - path: <путь-поля>
      level: pod|container|initContainer
      description: "<что делает это поле>"
  speFields:                              # Только если SPE поддерживается
    - path: <путь-поля-spe>
      level: pod|container
      description: "<что делает это SPE-поле>"
  applicableTracks:
    functional: true
    spePod: true|false
    speContainer: true|false
```

### Шаг 3: Создайте файлы constraint

Создайте директорию `constraints/` с соответствующими манифестами constraint:
- `pss_baseline.yaml` для baseline-тестов
- `policy_1.yaml` для тестов security policy

### Шаг 4: Создайте test-matrix.yaml

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestMatrix
metadata:
  name: <ваш-constraint>
spec:
  suiteName: d8-<ваш-constraint>
  outputTestDirectory: rendered
  defaultObjectBase: admissionPod
  defaultInventory:
    - ref: ../../../_test-samples/ns.yaml
  bases:
    admissionPod:
      document:
        apiVersion: v1
        kind: Pod
        metadata:
          namespace: testns
        spec:
          containers:
            - image: nginx
              name: nginx
    securityPolicyException:              # Только если SPE поддерживается
      document:
        apiVersion: deckhouse.io/v1alpha1
        kind: SecurityPolicyException
        metadata:
          namespace: testns
  blocks:
    - name: <имя-блока>
      gatorBlock: <имя-блока>
      template: ../rendered/constraint-template.yaml
      constraint: constraints/<файл-constraint>.yaml
      cases:
        - name: <имя-кейса>
          violations: "yes" #|"no"
          fields:
            - path: <путь-поля>
              scenario: <сценарий>
          object:
            base: admissionPod
            merge:
              metadata:
                name: <имя-кейса>
              spec: ...
```

### Шаг 5: Создайте test_profile.yaml

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestProfile
metadata:
  name: <ваш-constraint>
spec:
  testDirectory: <ваш-constraint>
  suite:
    expectedTestBlockNames:
      - <имя-блока-из-матрицы>
```

### Шаг 6: Сгенерируйте, проверьте, протестируйте

```bash
# Из директории constraint:
constraint_testgen=../../../../tools/constraint_testgen

# Генерация артефактов
go run $constraint_testgen generate -bundle ./test-matrix.yaml

# Проверка профиля
go run $constraint_testgen verify

# Запуск gator
gator verify -v ./rendered

# Проверка покрытия
go run $constraint_testgen coverage -tests-root ./ -format table
```

### Шаг 7: Итерируйтесь

Повторяйте generate → проверка rendered → coverage → gator, пока:
- Не исчезнут пропущенные обязательные сценарии
- Все тесты gator не пройдут
- Поведение тестов не будет соответствовать замыслу политики

---

## 10. Пошагово: добавление тестов к существующему constraint

### Шаг 1: Оцените текущее состояние

```bash
# Проверьте текущее покрытие
go run $constraint_testgen coverage -tests-root ./ -format table
```

Найдите пропущенные сценарии в выводе.

### Шаг 2: Добавьте кейсы в test-matrix.yaml

Для каждого пропущенного сценария добавьте новый кейс (или добавьте аннотации `fields` к существующим кейсам):

```yaml
- name: <описательное-имя-кейса>
  violations: "yes"|"no"
  fields:
    - path: <путь-поля-из-test_fields>
      scenario: <пропущенный-сценарий>
  object:
    base: admissionPod
    merge:
      metadata:
        name: <описательное-имя-кейса>
      spec: ...
```

### Шаг 3: Обновите test_fields.yaml при необходимости

Если Rego был обновлён для проверки новых полей, добавьте их в `test_fields.yaml`.

### Шаг 4: Перегенерируйте и проверьте

```bash
go run $constraint_testgen generate -bundle ./test-matrix.yaml
gator verify -v ./rendered
go run $constraint_testgen coverage -tests-root ./ -format table
```

---

## 11. Полезные команды

Все команды предполагают, что вы находитесь в **корне модуля** (`modules/015-admission-policy-engine`):

```bash
# Путь к инструменту
constraint_testgen=./tools/constraint_testgen

# Генерация артефактов из матрицы (один constraint)
go run $constraint_testgen generate \
  -bundle ./charts/constraint-templates/tests/test_cases/constraints/<group>/<constraint>/test-matrix.yaml

# Генерация всех constraint-ов сразу
go run $constraint_testgen generate -all \
  -tests-root ./charts/constraint-templates/tests/test_cases/constraints

# Проверка профилей constraint
go run $constraint_testgen verify \
  -tests-root ./charts/constraint-templates/tests/test_cases/constraints

# Проверка покрытия (формат таблицы)
go run $constraint_testgen coverage \
  -tests-root ./charts/constraint-templates/tests/test_cases/constraints -format table

# Проверка покрытия (формат JSON)
go run $constraint_testgen coverage \
  -tests-root ./charts/constraint-templates/tests/test_cases/constraints -format json

# Запуск gator для одного constraint
cd ./charts/constraint-templates/tests/test_cases/constraints/<group>/<constraint>
gator verify -v ./rendered

# Запуск всех тестов (OPA library + gator + coverage)
./charts/constraint-templates/tests/test_cases/run_all_tests.sh
```

### Из директории constraint

```bash
# Путь к инструменту (относительный из директории constraint)
constraint_testgen=../../../../tools/constraint_testgen

# Генерация
go run $constraint_testgen generate -bundle ./test-matrix.yaml

# Проверка
go run $constraint_testgen verify

# Gator
gator verify -v ./rendered

# Покрытие
go run $constraint_testgen coverage -tests-root ./ -format table
```

### Необходимые инструменты

Должны быть установлены:
- `go` — компилятор Go (для запуска constraint_testgen)
- `gator` — CLI OPA Gatekeeper (`go install github.com/open-policy-agent/gatekeeper/v3/cmd/gator@latest`)
- `opa` — CLI Open Policy Agent (для тестов OPA-библиотек)
- `python3` — используется скриптом запуска для парсинга покрытия

---

## 12. Известные ограничения

1. **Семантика мержа массивов**: массивы в патчах `merge` **заменяют** весь массив по этому пути. Используйте `containerMerges` / `initContainerMerges` для точечных патчей контейнеров.

2. **Сгенерированные файлы**: никогда не редактируйте файлы в `rendered/` вручную. Они перезаписываются при каждом запуске `generate`.

### Тестирование constraint-ов с external_data

Constraint-ы, использующие `external_data` (например, `verify-image-signature`, `vulnerable-images`), **могут** быть протестированы через gator с помощью паттерна мок-данных через inventory. Подход:

1. **Rego-шаблон** включает параметр `isTest`. Когда `isTest: true`, шаблон вызывает `external_data_from_inventory(provider, keys)` вместо реальной функции `external_data`. Этот хелпер читает мок-ответы из inventory-объектов gator.

2. **Манифест constraint** устанавливает `parameters.isTest: true`:

   ```yaml
   spec:
     parameters:
       isTest: true
   ```

3. **test-matrix.yaml** декларирует мок-ответы провайдеров на двух уровнях:
   - `spec.externalData.providers` — мок по умолчанию для всех кейсов (копируется в каждый сгенерированный кейс)
   - Per-case `externalData.providers` — переопределение для конкретных кейсов (например, для симуляции ошибок или уязвимостей)

   ```yaml
   spec:
     externalData:
       providers:
         - name: trivy-provider
           errors: []
           system_error: ""
           responses:
             "nginx:latest":
               vulnerabilities: []
     blocks:
       - name: security-policy
         cases:
           - name: negative-image-reference
             violations: "yes"
             externalData:                    # Per-case переопределение
               providers:
                 - name: trivy-provider
                   errors:
                     - "image contains high vulnerabilities"
                   system_error: ""
                   responses:
                     "vulnerable/nginx:latest":
                       vulnerabilities:
                         - severity: HIGH
                           id: CVE-TEST-0001
             object:
               base: admissionPod
               merge: ...
   ```

Полные рабочие примеры: `test_cases/constraints/security/vulnerable-images/test-matrix.yaml` и `test_cases/constraints/security/verify-image-signature/test-matrix.yaml`.

---

## 13. Устранение неполадок

### Coverage сообщает, что сценарий пропущен, но кейс есть

Чаще всего причина — несовпадение `fields.path` или неправильное значение `scenario`. Путь должен побайтово совпадать с путём из `test_fields.yaml`.

### Кейс проверяет поведение, но coverage всё равно низкий

Вероятно, у кейса нет аннотации `fields` (или она неполная). Coverage считается по аннотациям.

### Сгенерированные файлы изменились «неожиданно»

После правок матрицы это нормально. Рассматривайте `rendered/` как build output. Проверяйте корректность, но не «чините» YAML вручную.

### SPE-кейсы неожиданно падают

Проверьте повторно:
- Корректность SPE-пути относительно CRD
- Действительно ли кейс совпадает по SPE selector/target
- Что кейс сопоставлен именно со SPE-сценариями (`speMatch`, `speMismatch`, `speAbsent`, `speContainerSpecific`)
- Что Pod имеет корректный лейбл `security.deckhouse.io/security-policy-exception`

### gator падает, хотя матрица выглядит корректно

Убедитесь, что после последних правок повторно запускалась генерация и `rendered/` актуален. Затем проверьте проблемный fixture в `rendered/test_samples/` и сравните с ожидаемыми base/merge входами.

---

## 14. Definition of Done

Считайте constraint завершённым только если все пункты истинны:

- [ ] `test_profile.yaml` существует и определяет обязательные блоки suite в `spec.suite.expectedTestBlockNames`
- [ ] `test_fields.yaml` содержит все поля объекта, влияющие на решение политики в Rego
- [ ] `test_fields.yaml` содержит все поля SPE, используемые constraint-ом
- [ ] `level`, `defaultBehavior` и `applicableTracks` заданы корректно
- [ ] Для каждого обязательного сценария каждого поля есть хотя бы один кейс в `test-matrix.yaml`
- [ ] `constraint_testgen generate` выполняется успешно, `rendered/` обновлён
- [ ] `constraint_testgen verify` проходит (профиль валиден, обязательные блоки присутствуют)
- [ ] Coverage не показывает пропущенных обязательных сценариев
- [ ] Проверка gator проходит успешно
- [ ] Имена кейсов и аннотации `fields` понятны ревьюеру без контекста
