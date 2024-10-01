---
title: "Версионирование модуля"
permalink: ru/module-development/versioning/
lang: ru
---

Для версионирования модулей используется [семантическое версионирование](https://semver.org/lang/ru/).

При выборе номера версии используйте следующие рекомендации:
- изменение **patch-версии** (например, c `0.0.1` на `0.0.2`) — исправление дефекта;
- изменение **Minor-версии** (например, c `0.0.1` на `0.1.0`) — добавление новой функции;
- изменение **Major-версии** (например, c `0.0.1` на `1.0.0`) — добавление функции, которая кардинально меняет возможности модуля; масштабное изменение интерфейса или завершение крупного этапа работы.

Перед номером версии в теге git и контейнере registry **всегда** добавляется буква "v". Например: `v0.0.73`, `v1.0.0`.

## Каналы обновлений

Версия модуля при публикации должна оставаться неизменной на всех [каналах обновлений](../../deckhouse-release-channels.html) от менее стабильного к более стабильному: `Alpha` → `Beta` → `EarlyAccess` → `Stable` → `RockSolid`.

Каналы обновлений позволяют опубликовать версию модуля для ограниченного числа пользователей и получить обратную связь на раннем этапе. Вы сами определяете степень стабильности версии модуля и на какой канал обновлений ее можно опубликовать.

Важно понимать, что выбор канала обновлений не определяет, насколько стабилен модуль и на какой стадии жизненного цикла он находится. Каналы являются инструментом доставки и определяют степень стабильности конкретного релиза.

## Жизненный цикл модуля

Во время разработки модуль может находиться на следующих этапах:

**Experimental** — экспериментальная версия. Функциональность модуля может сильно измениться. Совместимость с будущими версиями не гарантируется.

**Preview** — предварительная версия. Функциональность модуля может измениться, но основные возможности сохранятся. Совместимость с будущими версиями обеспечивается, но может потребовать дополнительных действий по миграции.

**General Availability (GA)** — общедоступная версия. Модуль готов к использованию в production-средах.

**Deprecated** — модуль устарел, поддержка прекращена.

## Как понять, насколько модуль стабилен?

В зависимости от этапа жизненного цикла модуля и канала обновлений, из которого была установлена версия модуля, общая стабильность может быть определена в соответствии со следующей таблицей:

<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Этапы модулей</title>
    <style>
        body {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        table {
            width: 100%;
            table-layout: fixed;
            border-collapse: collapse;
            margin: 20px auto;
            font-size: 0.7em;
        }
        th, td {
            padding: 6px;
            border: 1px solid #000;
            text-align: center;
            vertical-align: middle;
            word-wrap: break-word;
        }
        th {
            background-color: #f2f2f2;
            font-weight: bold;
            text-align: center;
            vertical-align: middle;
        }
        .header-row {
            background-color: #e0e0e0;
            font-weight: bold;
        }
        .sub-header {
            background-color: #f9f9f9;
        }
        .pink {
            background-color: #ffe6e6;
        }
        .yellow {
            background-color: #ffebcc;
        }
        .green {
            background-color: #d9ead3;
        }
        .grey {
            background-color: #eeeeee;
        }
        .medium-green {
            background-color: #89AC76;
        }
        .dark-green {
            background-color: #44944A;
        }
    </style>
</head>
<body>

<table>
    <thead>
        <tr class="header-row">
            <th rowspan="2" style="text-align:center; vertical-align: middle;">Этап</th>
            <th colspan="5" style="text-align:center; vertical-align: middle;">Каналы обновлений</th>
        </tr>
        <tr class="sub-header">
            <th style="text-align:center; vertical-align: middle;">Alfa</th>
            <th style="text-align:center; vertical-align: middle;">Beta</th>
            <th style="text-align:center; vertical-align: middle;">EarlyAccess</th>
            <th style="text-align:center; vertical-align: middle;">Stable</th>
            <th style="text-align:center; vertical-align: middle;">RockSolid</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td style="text-align:center; vertical-align: middle;"><strong>Experimental (Экспериментальный)</strong></td>
            <td class="pink" style="text-align:center; vertical-align: middle;">Эксперименты, проверка функциональности</td>
            <td class="pink" style="text-align:center; vertical-align: middle;">Эксперименты, проверка функциональности</td>
            <td class="pink" style="text-align:center; vertical-align: middle;">Эксперименты, проверка функциональности</td>
            <td class="yellow" style="text-align:center; vertical-align: middle;">Эксперименты, проверка функциональности. Точечное использование опытными пользователями в окружениях, приравненных к продуктивным</td>
            <td class="yellow" style="text-align:center; vertical-align: middle;">Эксперименты, проверка функциональности. Точечное использование опытными пользователями в окружениях, приравненных к продуктивным</td>
        </tr>
        <tr>
            <td style="text-align:center; vertical-align: middle;"><strong>Preview (Предварительный)</strong></td>
            <td class="pink" style="text-align:center; vertical-align: middle;">Эксперименты, проверка функциональности</td>
            <td class="yellow" style="text-align:center; vertical-align: middle;">Окружения разработки, пилоты, не критично важные продуктивные окружения</td>
            <td class="yellow" style="text-align:center; vertical-align: middle;">Окружения разработки, пилоты, не критично важные продуктивные окружения</td>
            <td class="green" style="text-align:center; vertical-align: middle;">Продуктивные окружения и приравненные к ним</td>
            <td class="green" style="text-align:center; vertical-align: middle;">Продуктивные окружения и приравненные к ним</td>
        </tr>
        <tr>
            <td style="text-align:center; vertical-align: middle;"><strong>GA (Общедоступный)</strong></td>
            <td class="pink" style="text-align:center; vertical-align: middle;">Эксперименты, проверка функциональности</td>
            <td class="yellow" style="text-align:center; vertical-align: middle;">Окружения разработки, пилоты, не критичные продуктивные окружения</td>
            <td class="green" style="text-align:center; vertical-align: middle;">Окружения разработки, пилоты, не критичные продуктивные окружения</td>
            <td class="medium-green" style="text-align:center; vertical-align: middle;">Продуктивные окружения и приравненные к ним</td>
            <td class="dark-green" style="text-align:center; vertical-align: middle;">Критично важные продуктивные окружения и приравненные к ним</td>
        </tr>
        <tr>
            <td style="text-align:center; vertical-align: middle;"><strong>Deprecated (Выведен из использования)</strong></td>
            <td class="grey" style="text-align:center; vertical-align: middle;">Необходимо выводить из использования</td>
            <td class="grey" style="text-align:center; vertical-align: middle;">Необходимо выводить из использования</td>
            <td class="grey" style="text-align:center; vertical-align: middle;">Необходимо выводить из использования</td>
            <td class="grey" style="text-align:center; vertical-align: middle;">Необходимо выводить из использования</td>
            <td class="grey" style="text-align:center; vertical-align: middle;">Необходимо выводить из использования</td>
        </tr>
    </tbody>
</table>

</body>
</html>

Выводы:
- Модуль на стадии `Experimental` в канале `Stable` не рекомендуется использовать в production-средах.
- Модуль на стадии `GA` в канале `Alpha` также не рекомендуется использовать в production-средах.
- Для production-сред подходят только модули, находящиеся на стадии `GA`, установленные из каналов `EarlyAccess`, `Stable`, или `RockSolid`.
- Модули, находящиеся на стадии `Deprecated`, рекомендуется заменить.

<!--
## Стадии отдельных возможностей модуля @TODO

Ресурс *ModuleConfig* позволяет управлять дополнительными возможностями модуля. Эти опции могут быть помечены как `Experimental`, `Preview`, `GA` или `Deprecated` в параметре `x-feature-stage` в схеме OpenAPI `x-feature-stage: Experimental|Preview|GA|Deprecated` (значение по умолчанию — `GA`).

При включении функций на стадии, отличной от `GA`, выдается предупреждение.

В настройках Deckhouse Kubernetes Platform (DKP) можно задать глобальные правила, определяющие, какие функции и на каком этапе могут быть включены в кластере. Это помогает предотвратить случайное использование Experimental-функций в рабочих средах.
-->

## Версионирование API

Модули в DKP используют кастомные ресурсы для взаимодействия с пользователями. Параметр `apiVersion` с версией API этих ресурсов обновляется в соответствии со следующими правилами:

- `v1alphaX` — только что опубликованный API. Нужно проверить, насколько он удобен и понятен для пользователей, а также насколько корректны и логичны его настройки.
- `v1betaX` — API прошел первичное тестирование. Продолжается его логическое развитие и доработка.
- `v1stableX` — стабильный API. С этого момента его поля не удаляются из спецификации и правила валидации не меняются в сторону большей строгости.

Можно выпустить новую версию API v2, которая проходит те же этапы, но с префиксом `v2`. Важно помнить, что после выпуска версии `v1stableX` Kubernetes будет считать её более приоритетной, чем `alpha`- или `beta`-версии, до выпуска новой стабильной версии `v2stableX`. При выполнении команд `kubectl apply` и `kubectl edit` будет использоваться именно `v1stableX`.

Причины для выпуска новой версии:
* изменение структуры;
* обновление устаревших параметров.

Добавлять новые параметры можно без изменения версии.

Для автоматической конвертации параметров модуля из одной версии в другую — включите в модуль соответствующие [конверсии](../structure/#conversions).
Это может понадобиться, например, при переименовании или перемещении параметра в новой версии OpenAPI-спецификации.

При выходе новой версии *CustomResourceDefinition* (CRD) используйте следующие рекомендации:
* Предыдущим версиям проставлять параметр `deprecated: true`, подробнее в документации [Kubernetes](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#version-deprecation).
* Версию, в которой данные хранятся внутри etcd ([storage-версия](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#upgrade-existing-objects-to-a-new-stored-version)), менять не ранее чем через два месяца после выхода новой версии.
