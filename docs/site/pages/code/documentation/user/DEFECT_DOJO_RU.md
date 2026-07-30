---
title: "Интеграция с DefectDojo"
menuTitle: DefectDojo
force_searchable: true
description: "Настройка интеграции Deckhouse Code с системой управления уязвимостями DefectDojo."
permalink: ru/code/documentation/user/defect-dojo.html
lang: ru
weight: 50
---

Интеграция с DefectDojo позволяет собирать уязвимости из security-отчётов Deckhouse Code в единой системе управления уязвимостями.

![Форма интеграции DefectDojo](/images/code/defect_dojo_integration_form_en.png)

## Предварительные требования

Перед включением интеграции убедитесь, что:

- У вас есть доступный экземпляр DefectDojo.
- У вас есть API-токен DefectDojo с доступом к импорту сканов.
- У вас есть роль **Maintainer** в проекте Deckhouse Code.
- Ваш CI-конвейер формирует поддерживаемые security-артефакты.

## Включение интеграции DefectDojo

Чтобы настроить интеграцию:

1. Откройте проект в Deckhouse Code.
1. Перейдите в **Settings** → **Integrations**.
1. Откройте **DefectDojo**.
1. Заполните поля интеграции:
   - **URL**.
   - **API token**.
   - **Product name** (необязательно; по умолчанию используется полный путь проекта).
   - **Product type name**.
   - **Engagement name** (необязательно; по умолчанию используется имя ветки).
   - **Minimum severity**.
   - **Auto-create context** (`auto_create_context`).
   - **Imported findings (active)** (`findings_active`).
   - **Verified findings** (`findings_verified`).
   - **Close old findings** (`close_old_findings`).
1. Нажмите **Save changes**.

## Проверка параметров подключения

Нажмите **Test settings** на странице интеграции DefectDojo, чтобы проверить доступность DefectDojo и корректность токена.

## Как работает автоматическая загрузка

После завершения сборки, когда доступны security-артефакты, Deckhouse Code запускает `FE::Security::DefectDojoUploadWorker` и отправляет отчёты в DefectDojo через endpoint reimport API `POST /api/v2/reimport-scan/`.

Интеграция загружает отчёты следующих сканеров:

- SAST
- Secret detection
- Dependency scanning
- Container scanning
- DAST

## Сопоставление сущностей в DefectDojo

По умолчанию Deckhouse Code использует следующие соответствия:

- **Product** = полный путь проекта (или заданное Product name).
- **Engagement** = имя ветки (или заданное Engagement name).
- **Test** = имя CI-задачи.
- **test_title** = имя CI-задачи.

## Параметры импорта по умолчанию

Deckhouse Code передаёт параметры импорта согласно настройкам интеграции:

- Порог минимальной критичности.
- `auto_create_context`.
- Статус active для импортированных находок.
- Статус verified для импортированных находок.
- `close_old_findings`.

## Безопасность учётных данных в CI

Если вы используете встроенные CI-переменные для интеграции с DefectDojo (`DD_URL` и `DD_TOKEN`), пометьте их как **masked** и **protected**.

## Находки в DefectDojo

На скриншоте ниже показаны находки, импортированные в DefectDojo.

![Находки в DefectDojo](/images/code/defect_dojo_findings_en.png)
