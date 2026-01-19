# Использование данных из oss.yaml в документации

## Обзор

Данные из файлов `oss.yaml` автоматически собираются и доступны в документации через `site.data.oss`.

## Структура данных

Данные организованы в следующем формате:

```yaml
<module-name1>:
  - name: Component Name
    link: https://example.com
    description: Component description
    logo: https://example.com/logo.png
    license: Apache License 2.0
    id: component-id
    version: 1.0.0
  - name: Another Component
    ...
<module-name2>:
  - name: ...
```

## Использование в Jekyll

### Доступ к данным

Данные доступны через `site.data.oss[module_name]`, где `module_name` - это имя модуля без префикса номера (например, `cert-manager` вместо `101-cert-manager`).

### Пример 1: Базовое использование

```liquid
{%- assign module_name = "cert-manager" -%}
{%- if site.data.oss[module_name] -%}
  {%- assign oss_items = site.data.oss[module_name] -%}
  <ul>
    {%- for item in oss_items -%}
      <li>
        <a href="{{ item.link }}">{{ item.name }}</a>
        {%- if item.version -%}
          (v{{ item.version }})
        {%- endif -%}
      </li>
    {%- endfor -%}
  </ul>
{%- endif -%}
```

### Пример 2: Использование include файла

Используйте готовый include файл `_includes/oss_data_example.liquid`:

```liquid
{%- include oss_data_example.liquid module_name="cert-manager" -%}
```

Или установите `module_name` в front matter страницы:

```yaml
---
title: Module Documentation
module_name: cert-manager
---
```

И затем используйте:

```liquid
{%- include oss_data_example.liquid -%}
```

### Пример 3: Поиск компонента по ID

```liquid
{%- assign module_name = "cert-manager" -%}
{%- assign component_id = "cert-manager" -%}
{%- if site.data.oss[module_name] -%}
  {%- for item in site.data.oss[module_name] -%}
    {%- if item.id == component_id -%}
      <p>Version: {{ item.version }}</p>
      <p>License: {{ item.license }}</p>
    {%- endif -%}
  {%- endfor -%}
{%- endif -%}
```

## Генерация данных

Данные автоматически генерируются при сборке документации:

1. **Локально**: При выполнении `make generate` или `make generate-docs`
2. **В werf**: Автоматически при сборке образа документации

Скрипт `docs/documentation/_tools/generate_oss_data.sh` собирает все файлы `oss.yaml` из модулей и создает файл `docs/documentation/_data/oss.yaml`.

## Формат файла oss.yaml

Каждый модуль может содержать файл `oss.yaml` в своей корневой директории:

```yaml
- name: Component Name
  link: https://github.com/example/component
  description: Component description
  logo: https://example.com/logo.png
  license: Apache License 2.0
  id: component-id
  version: 1.0.0
```

Все поля, кроме `name`, являются опциональными.

## Соответствие с werf

Структура данных в документации соответствует структуре, используемой в werf через шаблон `.werf/defines/oss_yaml.tmpl`. Это позволяет использовать одни и те же данные как в сборке образов, так и в документации.
