# Генерация OSS данных для документации

## Описание

Скрипт `generate_oss_data.sh` собирает данные из всех файлов `oss.yaml` в модулях и создает единый YAML файл для использования в документации Jekyll.

## Использование

### Локально

```bash
export OSS_SOURCE_DIR=.
export OSS_OUTPUT_FILE=./docs/documentation/_data/oss.yaml
bash ./docs/documentation/_tools/generate_oss_data.sh
```

Или через Makefile:

```bash
make generate-docs
```

### В werf

Скрипт автоматически вызывается при сборке документации через werf в файлах:
- `docs/documentation/werf-documentation-static.inc.yaml`
- `docs/documentation/werf-modules-static.inc.yaml`

## Формат выходных данных

Скрипт создает файл `docs/documentation/_data/oss.yaml` со следующей структурой:

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

Где `module-name` - это имя модуля без префикса номера (например, `cert-manager` вместо `101-cert-manager`).

## Соответствие с werf

Структура данных соответствует структуре, используемой в werf через шаблон `.werf/defines/oss_yaml.tmpl`. Это позволяет использовать одни и те же данные как в сборке образов, так и в документации.

## Использование в документации

Данные доступны через `site.data.oss[module_name]` в Jekyll шаблонах.

Примеры использования см. в файле `OSS_DATA_USAGE.md`.
