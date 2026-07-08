---
title: "Интеграция CodeScoring"
menuTitle: CodeScoring
force_searchable: true
description: Настройка интеграции CodeScoring SCA/OSA-сканера в Deckhouse Code для проверки зависимостей и уязвимостей
permalink: ru/code/documentation/user/codescoring.html
lang: ru
weight: 90
---

CodeScoring — это инструмент анализа состава программного обеспечения (SCA/OSA) для проверки сторонних зависимостей на наличие уязвимостей, лицензионных рисков и нарушений политик безопасности.

{% alert level="info" %}
Интеграция CodeScoring в Deckhouse Code покрывает SCA/OSA-сценарии: анализ зависимостей, обнаружение уязвимостей, SBOM-генерация и применение политик безопасности.
SAST и DAST в эту интеграцию не входят.
{% endalert %}

## Возможности интеграции

- Анализ зависимостей приложения (пакеты, библиотеки, версии).
- Обнаружение уязвимостей по базам CVE (включая ФСТЭК БДУ и Kaspersky OSS feed).
- Генерация SBOM в формате CycloneDX.
- Применение политик безопасности с блокировкой или предупреждением в CI.
- Нативный вывод отчётов в формате GitLab Dependency Scanning и Code Quality для отображения в MR-виджетах.

## Предварительные требования

Перед настройкой интеграции убедитесь, что:

- У вас развёрнут сервер CodeScoring (on-prem или SaaS).
- Вы получили API-токен в профиле пользователя CodeScoring.
- Агент Johnny доступен в CI-окружении (Docker-образ или бинарный файл).

Подробнее о развёртывании сервера CodeScoring см. в официальной документации: [docs.codescoring.ru](https://docs.codescoring.ru/on-premise/).

## Настройка интеграции в проекте

Параметры подключения к CodeScoring задаются через настройки проекта.

1. Откройте проект в Deckhouse Code.
2. Перейдите в **Настройки** → **Интеграции**.
3. Найдите раздел **CodeScoring** и нажмите **Настроить**.
4. Заполните параметры подключения:

| Параметр | Описание |
|----------|----------|
| **URL сервера** | Адрес сервера CodeScoring, например `https://codescoring.example.com` |
| **API-токен** | Токен из профиля пользователя CodeScoring |
| **Название проекта** | Имя проекта в CodeScoring (по умолчанию — имя репозитория) |
| **Стадия сканирования** | Стадия CI для привязки результатов: `build`, `dev`, `stage`, `test`, `prod` (по умолчанию `build`) |
| **Включить интеграцию** | Переключатель активации интеграции для проекта |

5. Нажмите **Сохранить**.

## Конфигурация CI-пайплайна

После настройки интеграции подключите шаблон CodeScoring в `.gitlab-ci.yml` проекта.

### Подключение шаблона

```yaml
include:
  - project: "deckhouse/code/gitlab-custom"
    file: ".gitlab/ci/includes/codescoring.gitlab-ci.yml"

variables:
  CODESCORING_ENABLED: "true"
  CODESCORING_URL: $CODESCORING_URL         # задайте в CI/CD Variables
  CODESCORING_TOKEN: $CODESCORING_TOKEN     # задайте как masked переменную
  CODESCORING_PROJECT: $CI_PROJECT_NAME
  CODESCORING_SCAN_STAGE: "build"
  CODESCORING_POLICY_MODE: "blocking"       # или "warning"
```

Переменные `CODESCORING_URL` и `CODESCORING_TOKEN` рекомендуется задавать через **Settings → CI/CD → Variables**, отмечая токен как `Masked`.

### Стадии пайплайна

Интеграция добавляет следующие стадии:

| Job | Стадия | Описание |
|-----|--------|----------|
| `codescoring-sbom` | `.pre` | Генерация SBOM в формате CycloneDX. Артефакт передаётся следующим job |
| `codescoring-dependency-scan` | `security` | Анализ зависимостей, вывод GitLab Dependency Scanning Report |
| `codescoring-code-quality-scan` | `security` | Проверка качества кода, вывод GitLab Code Quality Report |
| `codescoring-build-scan` | `security` | Анализ артефактов сборки (опционально, требует `CODESCORING_BUILD_PATH`) |

Задачи сканирования запускаются **параллельно** после генерации SBOM, что сокращает общее время проверки.

## SBOM-пред-стадия

Перед запуском сканирования автоматически выполняется генерация SBOM (Software Bill of Materials) в формате CycloneDX:

- SBOM фиксирует точный состав зависимостей на момент сборки.
- Один SBOM используется несколькими задачами анализа.
- Артефакт доступен для повторного использования другими инструментами.

Если SBOM уже существует как артефакт предыдущей стадии, повторная генерация пропускается.

## Режимы политик

### Блокирующий режим (blocking)

Пайплайн завершается с ошибкой при нарушении политики (exit code 1):

```yaml
variables:
  CODESCORING_POLICY_MODE: "blocking"
```

Рекомендуется для защищённых веток и release-контуров.

### Предупреждающий режим (warning)

Результаты публикуются как предупреждения, пайплайн не останавливается:

```yaml
variables:
  CODESCORING_POLICY_MODE: "warning"
```

Рекомендуется для пилотного внедрения или feature-веток.

## Отображение результатов в Merge Request

После выполнения сканирования результаты отображаются в виджетах MR:

- **Security scanning** — найденные уязвимости с деталями CVE, severity и рекомендациями.
- **Code Quality** — нарушения качественных метрик.

Виджеты появляются автоматически при наличии артефактов `gl-dependency-scanning-report.json` и `gl-code-quality-report.json`.

## Триаж уязвимостей

Обнаруженные уязвимости можно разбирать непосредственно в интерфейсе CodeScoring:

- Переход в **SCA → Уязвимости**.
- Установка статуса: `Активен`, `Подтверждён`, `Не затронут`, `Ложноположительный`.
- Заполнение обоснования и ответа (совместимо с форматом CycloneDX VEX).

Временное игнорирование срабатываний возможно по проекту, технологии, пакету, лицензии или CVE.

## Развёртывание сервера CodeScoring

Для self-hosted установки обратитесь к инструкциям:

- [Docker Compose](deployment-docker.html) — развёртывание на одном сервере.
- [Kubernetes/Helm](https://docs.codescoring.ru/on-premise/kubernetes/) — production-окружение.

Системные требования см. на странице [docs.codescoring.ru/on-premise/requirements/](https://docs.codescoring.ru/on-premise/requirements/).

## Устранение неполадок

### Сканирование не запускается

Проверьте:

- Переменная `CODESCORING_ENABLED` установлена в `"true"`.
- Переменные `CODESCORING_URL` и `CODESCORING_TOKEN` заданы и доступны раннеру.
- Шаблон подключён в `.gitlab-ci.yml`.

### Пайплайн блокируется при нарушении политики

Это ожидаемое поведение в режиме `blocking`. Для временного отключения блокировки:

- Переключите `CODESCORING_POLICY_MODE: "warning"`, или
- Устраните нарушение через триаж в интерфейсе CodeScoring.

### Виджеты безопасности не отображаются в MR

Проверьте:

- Артефакты `gl-dependency-scanning-report.json` и `gl-code-quality-report.json` созданы.
- Секция `artifacts.reports` в job-конфигурации указана корректно.
- Job завершился (даже с ошибкой — артефакты собираются при `when: always`).
