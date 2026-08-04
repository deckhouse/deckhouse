---
title: "Интеграция с DefectDojo"
menuTitle: Интеграция с DefectDojo
force_searchable: true
description: "Настройка интеграции Deckhouse Code с системой управления уязвимостями DefectDojo."
permalink: ru/code/documentation/user/defect-dojo.html
lang: ru
weight: 50
---

Эта интеграция позволяет автоматически импортировать результаты проверок безопасности из Deckhouse Code в DefectDojo, единую систему управления уязвимостями.

![Форма интеграции DefectDojo](/images/code/defect_dojo_integration_form_en.png)

## Предварительные требования

Перед настройкой интеграции убедитесь, что:

- доступен экземпляр DefectDojo;
- получен API-токен DefectDojo с правом импорта результатов сканирования;
- в проекте Deckhouse Code настроена роль **Maintainer**;
- ваш CI-конвейер формирует поддерживаемые артефакты проверок безопасности.

## Настройка интеграции DefectDojo

Чтобы настроить интеграцию:

1. Откройте проект в Deckhouse Code.
1. Перейдите в раздел «Settings» → «Integrations» → «DefectDojo».
1. Заполните поля интеграции:
   - «URL» — адрес экземпляра DefectDojo;
   - «API token» — API-токен DefectDojo;
   - «Product name» (опционально) — имя продукта в DefectDojo. По умолчанию используется полный путь проекта;
   - «Product type name» — тип продукта в DefectDojo;
   - «Engagement name» (опционально) — имя Engagement. По умолчанию используется имя ветки;
   - «Minimum severity» — минимальная критичность импортируемых несоответствий;
   - «Auto-create context» (`auto_create_context`) — автоматически создавать отсутствующие сущности в DefectDojo;
   - «Imported findings (active)» (`findings_active`) — помечать импортированные несоответствия как активные;
   - «Verified findings» (`findings_verified`) — помечать импортированные несоответствия как подтверждённые;
   - «Close old findings» (`close_old_findings`) — закрывать отсутствующие в новом отчёте несоответствия.
1. Нажмите «Save changes».

### Проверка подключения

чтобы проверить доступность DefectDojo и корректность API-токена, нажмите «Test settings» на странице интеграции.

## Работа интеграции

### Автоматический импорт результатов сканирования

После завершения каждого CI-конвейера, сформировавшего отчеты проверок безопасности, Deckhouse Code автоматически импортирует их в DefectDojo через API `reimport-scan`.

Поддерживаются результаты следующих типов сканирования:

- SAST;
- Secret detection;
- Dependency scanning;
- Container scanning;
- DAST.

### Сопоставление сущностей в DefectDojo

По умолчанию Deckhouse Code использует следующие соответствия:

- **Product** = полный путь проекта (или заданное значение «Product name»);
- **Engagement** = имя ветки (или заданное значение «Engagement name»);
- **Test** = имя CI-задачи;
- **test_title** = имя CI-задачи.

### Параметры импорта

При импорте результатов Deckhouse Code передаёт в DefectDojo параметры согласно настройкам интеграции:

- «Minimum severity» (минимальная критичность импортируемых несоответствий);
- `auto_create_context`;
- `findings_active`;
- `findings_verified`;
- `close_old_findings`.

## Безопасность учётных данных в CI

Если вы используете встроенные CI-переменные `DD_URL` и `DD_TOKEN` для интеграции с DefectDojo, пометьте их как **masked** и **protected**.
