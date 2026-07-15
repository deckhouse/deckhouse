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
Интеграция CodeScoring в Deckhouse Code покрывает SCA/OSA-сценарии: анализ зависимостей, обнаружение уязвимостей, генерацию SBOM и триаж на стороне платформы.
SAST и DAST в эту интеграцию не входят.
{% endalert %}

## Возможности интеграции

- Анализ зависимостей приложения (пакеты, библиотеки, версии).
- Обнаружение уязвимостей по базам CVE (включая ФСТЭК БДУ и Kaspersky OSS feed).
- Генерация SBOM в формате CycloneDX.
- Отдача нативных отчётов GitLab за один прогон: Dependency Scanning, Code Quality, JUnit, а также SARIF и CycloneDX SBOM.
- Триаж и политики (пороги severity, игнорирование срабатываний) — на стороне платформы CodeScoring.

## Предварительные требования

Перед настройкой интеграции убедитесь, что:

- Развёрнут сервер CodeScoring (on-prem или SaaS).
- Получен API-токен в профиле пользователя CodeScoring.
- В проекте/группе доступен GitLab Runner с executor `docker` — джоба сканера выполняется в контейнере `debian:bookworm-slim`.

Подробнее о развёртывании сервера см. официальную документацию: [docs.codescoring.ru](https://docs.codescoring.ru/on-premise/).

{% alert level="info" %}
Агент Johnny **не нужно** устанавливать вручную: джоба сканера скачивает его с вашего сервера CodeScoring по API-токену при каждом запуске.
{% endalert %}

## Настройка интеграции в проекте

Параметры подключения к CodeScoring задаются через настройки проекта (или группы).

1. Откройте проект в Deckhouse Code.
2. Перейдите в **Настройки** → **Интеграции**.
3. Найдите раздел **CodeScoring** и откройте его.
4. Заполните параметры подключения:

| Параметр | Описание |
|----------|----------|
| **Активна** | Переключатель включения интеграции для проекта |
| **URL сервера** | Адрес сервера CodeScoring, например `https://codescoring.example.com` |
| **API-токен** | Токен из профиля пользователя CodeScoring (хранится в зашифрованном виде, маскируется) |
| **CA-сертификат** | Необязательный PEM-сертификат CA — для сервера CodeScoring с self-signed сертификатом |
| **Название проекта** | Имя проекта в CodeScoring (по умолчанию — slug репозитория) |
| **Стадия сканирования** | Стадия для привязки результатов на стороне платформы (по умолчанию `build`) |

5. Нажмите **Сохранить**.

Интеграция автоматически прокидывает в пайплайн CI-переменные `FE_SCANS_CODESCORING_URL`, `FE_SCANS_CODESCORING_TOKEN`, `FE_SCANS_CODESCORING_CA_CERT`, `FE_SCANS_CODESCORING_PROJECT`, `FE_SCANS_CODESCORING_SCAN_STAGE` — задавать их вручную в `.gitlab-ci.yml` не нужно.

## Запуск сканирования (scan-execution-политика)

Сканер подмешивается в пайплайн через **scan-execution-политику**, а не ручным `include`.

1. В проекте политик безопасности добавьте в `policy.yml` действие `codescoring`:

   ```yaml
   scan_execution_policy:
   - name: CodeScoring on every pipeline
     enabled: true
     rules:
     - type: pipeline
       branches: ["*"]
     actions:
     - scan: codescoring
   ```

2. Привяжите проект политик к целевому проекту: **Настройки** → **Security policy**.

После этого в каждый пайплайн автоматически добавляется джоба **`codescoring_scan`** (стадия `fe-security-scanner`), которая:

- скачивает агент Johnny с сервера CodeScoring (по токену; при self-signed — с переданным CA-сертификатом);
- сканирует рабочую директорию и отдаёт нативные отчёты GitLab.

Отдельный `include` и ручная установка переменных `CODESCORING_*` **не требуются** — всё подставляют интеграция и политика.

## Отчёты и где смотреть результаты

Одна джоба `codescoring_scan` формирует все отчёты за один прогон. Deckhouse Code основан на GitLab FOSS, где часть EE-виджетов отсутствует, поэтому результаты выводятся так:

| Отчёт | Где смотреть |
|-------|--------------|
| Тесты (JUnit) | вкладка **Tests** пайплайна (нативно) |
| Code Quality | виджет **Code Quality** в Merge Request (нативно) |
| Dependency Scanning | страница **CodeScoring**: `/-/security/codescoring` |
| SBOM (состав зависимостей) | страница **Dependency list**: `/-/security/dependencies` |
| Лицензии | страница **License compliance**: `/-/security/licenses` |
| SARIF | загружается артефактом (отдельного SAST-виджета в FOSS нет) |

{% alert level="info" %}
Страницы Dependency Scanning, Dependency list и License compliance — это FE-реализация Deckhouse Code: в апстрим-GitLab FOSS соответствующие виджеты доступны только в EE. Сейчас страницы открываются по прямому URL (пункт бокового меню — в планах).
{% endalert %}

## Политики и блокировка

Джоба `codescoring_scan` неблокирующая: она всегда завершается успешно и загружает отчёты (в том числе на упавших попытках — `artifacts:when: always`), не «роняя» пайплайн.

Настройка политик (40 критериев, пороги severity, триаж) и решение о блокировке выполняются на стороне платформы CodeScoring. Жёсткая блокировка пайплайна по нарушению политики — предмет отдельной настройки scan-execution-политики и в текущем шаблоне не включена.

## Триаж уязвимостей

Обнаруженные уязвимости можно разбирать непосредственно в интерфейсе CodeScoring:

- Переход в **SCA → Уязвимости**.
- Установка статуса: `Активен`, `Подтверждён`, `Не затронут`, `Ложноположительный`.
- Заполнение обоснования и ответа (совместимо с форматом CycloneDX VEX).

Временное игнорирование срабатываний возможно по проекту, технологии, пакету, лицензии или CVE.

{% alert level="warning" %}
На текущем этапе агент CodeScoring не заполняет `severity` в Dependency Scanning Report (только в Code Quality), поэтому на странице CodeScoring severity может отображаться как `unknown`.
{% endalert %}

## Развёртывание сервера CodeScoring

Для self-hosted установки используйте официальную документацию вендора:

- [Установка в Docker](https://docs.codescoring.ru/on-premise/docker/).
- [Установка в Kubernetes/Helm](https://docs.codescoring.ru/on-premise/kubernetes/).
- [Системные требования](https://docs.codescoring.ru/on-premise/requirements/).

## Устранение неполадок

### Сканирование не запускается

Проверьте:

- Интеграция **CodeScoring** активна в настройках проекта (заданы URL и токен).
- Проект политик привязан к проекту и содержит `- scan: codescoring`.
- В пайплайне присутствует джоба `codescoring_scan` и доступен раннер с docker-executor.

### Результаты не появляются на страницах CodeScoring / Dependency list / License compliance

Проверьте:

- Джоба `codescoring_scan` завершилась и загрузила артефакты `gl-dependency-scanning-report.json` и `gl-sbom.cdx.json` (артефакты собираются при `when: always`).
- Открывается страница ветки по умолчанию (страницы читают отчёт последнего пайплайна).

### Виджет Code Quality не отображается в Merge Request

Проверьте, что джоба сформировала `gl-code-quality-report.json` и он объявлен в секции `artifacts:reports:codequality`.
